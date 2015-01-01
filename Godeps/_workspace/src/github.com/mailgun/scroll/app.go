package scroll

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/gorilla/mux"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/manners"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll/vulcan"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll/vulcan/middleware"
)

const (
	// Suggested result set limit for APIs that may return many entries (e.g. paging).
	DefaultLimit = 100

	// Suggested max allowed result set limit for APIs that may return many entries (e.g. paging).
	MaxLimit = 10000

	// Suggested max allowed amount of entries that batch APIs can accept (e.g. batch uploads).
	MaxBatchSize = 1000

	// Interval between Vulcand heartbeats (if the app if configured to register in it).
	defaultRegisterInterval = 2 * time.Second
)

// Represents an app.
type App struct {
	Config   AppConfig
	router   *mux.Router
	registry *vulcan.Registry
	stats    *appStats
}

// Represents a configuration object an app is created with.
type AppConfig struct {
	// name of the app being created
	Name string

	// IP/port the app will bind to
	ListenIP   string
	ListenPort int

	// optional router to use
	Router *mux.Router

	// hostnames of the public and protected API entrypoints used for vulcan registration
	PublicAPIHost    string
	ProtectedAPIHost string
	ProtectedAPIURL  string

	// whether to register the app's endpoint and handlers in vulcan
	Register bool

	// metrics service used for emitting the app's real-time metrics
	Client metrics.Client
}

// Create a new app.
func NewApp() *App {
	return NewAppWithConfig(AppConfig{})
}

// Create a new app with the provided configuration.
func NewAppWithConfig(config AppConfig) *App {
	var reg *vulcan.Registry
	if config.Register != false {
		reg = vulcan.NewRegistry(vulcan.Config{
			PublicAPIHost:    config.PublicAPIHost,
			ProtectedAPIHost: config.ProtectedAPIHost,
		})
	}

	router := config.Router
	if router == nil {
		router = mux.NewRouter()
	}

	return &App{
		Config:   config,
		router:   router,
		registry: reg,
		stats:    newAppStats(config.Client),
	}
}

// Register a handler function.
//
// If vulcan registration is enabled in the both app config and handler spec,
// the handler will be registered in the local etcd instance.
func (app *App) AddHandler(spec Spec) error {
	var handler http.HandlerFunc

	// make a handler depending on the function provided in the spec
	if spec.RawHandler != nil {
		handler = spec.RawHandler
	} else if spec.Handler != nil {
		handler = MakeHandler(app, spec.Handler, spec)
	} else if spec.HandlerWithBody != nil {
		handler = MakeHandlerWithBody(app, spec.HandlerWithBody, spec)
	} else {
		return fmt.Errorf("the spec does not provide a handler function: %v", spec)
	}

	// register the handler in the router
	route := app.router.HandleFunc(spec.Path, handler).Methods(spec.Methods...)
	if len(spec.Headers) != 0 {
		route.Headers(spec.Headers...)
	}

	// vulcan registration
	if app.registry != nil && spec.Register != false {
		app.registerLocation(spec.Methods, spec.Path, spec.Scopes, spec.Middlewares)
	}

	return nil
}

// GetHandler returns HTTP compatible Handler interface.
func (app *App) GetHandler() http.Handler {
	return app.router
}

// SetNotFoundHandler sets the handler for the case when URL can not be matched by the router.
func (app *App) SetNotFoundHandler(fn http.HandlerFunc) {
	app.router.NotFoundHandler = fn
}

// IsPublicRequest determines whether the provided request came through the public HTTP endpoint.
func (app *App) IsPublicRequest(request *http.Request) bool {
	return request.Host == app.Config.PublicAPIHost
}

// Start the app on the configured host/port.
//
// If vulcan registration is enabled in the app config, starts a goroutine that
// will be registering the app's endpoint once every minute in the local etcd
// instance.
//
// Supports graceful shutdown on 'kill' and 'int' signals.
func (app *App) Run() error {
	http.Handle("/", app.router)

	if app.registry != nil {
		go func() {
			// heartbeat can be stopped/resumed on USR1 signal
			heartbeatChan := make(chan os.Signal, 1)
			signal.Notify(heartbeatChan, syscall.SIGUSR1)

			for {
				select {
				// this will proceed to the "default" without blocking if there is no signal
				case s := <-heartbeatChan:
					log.Infof("Got signal: %v, pausing heartbeat", s)
					// now it blocks until another signal comes
					<-heartbeatChan
					log.Infof("Resuming heartbeat")
				default:
					app.registerEndpoint()
					time.Sleep(defaultRegisterInterval)
				}
			}
		}()
	}

	// listen for a shutdown signal
	go func() {
		exitChan := make(chan os.Signal, 1)
		signal.Notify(exitChan, os.Interrupt, os.Kill)
		s := <-exitChan
		log.Infof("Got shutdown signal: %v", s)
		manners.Close()
	}()

	return manners.ListenAndServe(
		fmt.Sprintf("%v:%v", app.Config.ListenIP, app.Config.ListenPort), nil)
}

// registerEndpoint is a helper for registering the app's endpoint in vulcan.
func (app *App) registerEndpoint() {
	endpoint, err := vulcan.NewEndpoint(app.Config.Name, app.Config.ListenIP, app.Config.ListenPort)
	if err != nil {
		log.Errorf("Failed to create an endpoint: %v", err)
		return
	}

	if err := app.registry.RegisterEndpoint(endpoint); err != nil {
		log.Errorf("Failed to register an endpoint: %v %v", endpoint, err)
		return
	}

	if err := app.registry.RegisterServer(endpoint); err != nil {
		log.Errorf("Failed to register an endpoint: %v %v", endpoint, err)
		return
	}

	log.Infof("Registered: %v", endpoint)
}

// registerLocation is a helper for registering handlers in vulcan.
func (app *App) registerLocation(methods []string, path string, scopes []Scope, middlewares []middleware.Middleware) {
	for _, scope := range scopes {
		app.registerLocationForScope(methods, path, scope, middlewares)
	}
}

// registerLocationForScope registers a location with a specified scope.
func (app *App) registerLocationForScope(methods []string, path string, scope Scope, middlewares []middleware.Middleware) {
	host, err := app.apiHostForScope(scope)
	if err != nil {
		log.Errorf("Failed to register a location: %v", err)
		return
	}
	app.registerLocationForHost(methods, path, host, middlewares)
}

// registerLocationForHost registers a location for a specified hostname.
func (app *App) registerLocationForHost(methods []string, path, host string, middlewares []middleware.Middleware) {
	location := vulcan.NewLocation(host, methods, path, app.Config.Name, middlewares)

	if err := app.registry.RegisterLocation(location); err != nil {
		log.Errorf("Failed to register a location: %v %v", location, err)
		return
	}

	if err := app.registry.RegisterFrontend(location); err != nil {
		log.Errorf("Failed to register a frontend: %v %v", location, err)
		return
	}

	log.Infof("Registered: %v", location)
}

// apiHostForScope is a helper that returns an appropriate API hostname for a provided scope.
func (app *App) apiHostForScope(scope Scope) (string, error) {
	if scope == ScopePublic {
		return app.Config.PublicAPIHost, nil
	} else if scope == ScopeProtected {
		return app.Config.ProtectedAPIHost, nil
	} else {
		return "", fmt.Errorf("unknown scope value: %v", scope)
	}
}
