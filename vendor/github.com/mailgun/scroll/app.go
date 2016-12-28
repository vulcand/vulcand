package scroll

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/mailgun/log"
	"github.com/mailgun/manners"
	"github.com/mailgun/metrics"
	"github.com/mailgun/scroll/vulcand"
)

const (
	// Suggested result set limit for APIs that may return many entries (e.g. paging).
	DefaultLimit = 100

	// Suggested max allowed result set limit for APIs that may return many entries (e.g. paging).
	MaxLimit = 10000

	// Suggested max allowed amount of entries that batch APIs can accept (e.g. batch uploads).
	MaxBatchSize = 1000
)

// Represents an app.
type App struct {
	Config     AppConfig
	router     *mux.Router
	stats      *appStats
	vulcandReg *vulcand.Registry
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

	// Vulcand config must be provided to enable registration in Etcd.
	Vulcand *vulcand.Config

	// metrics service used for emitting the app's real-time metrics
	Client metrics.Client
}

// Create a new app.
func NewApp() (*App, error) {
	return NewAppWithConfig(AppConfig{})
}

// Create a new app with the provided configuration.
func NewAppWithConfig(config AppConfig) (*App, error) {
	app := App{Config: config}

	app.router = config.Router
	if app.router == nil {
		app.router = mux.NewRouter()
		app.router.UseEncodedPath()
	}
	app.router.HandleFunc("/_ping", handlePing).Methods("GET")

	if config.Vulcand != nil {
		var err error
		app.vulcandReg, err = vulcand.NewRegistry(*config.Vulcand, config.Name, config.ListenIP, config.ListenPort)
		if err != nil {
			return nil, err
		}
	}

	app.stats = newAppStats(config.Client)
	return &app, nil
}

// Register a handler function.
//
// If vulcan registration is enabled in the both app config and handler spec,
// the handler will be registered in the local etcd instance.
func (app *App) AddHandler(spec Spec) error {
	var handler http.HandlerFunc

	if spec.LogRequest == nil {
		spec.LogRequest = logRequest
	}

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

	for _, path := range spec.Paths {
		route := app.router.HandleFunc(path, handler).Methods(spec.Methods...)
		if len(spec.Headers) != 0 {
			route.Headers(spec.Headers...)
		}
		if app.vulcandReg != nil {
			app.registerFrontend(spec.Methods, path, spec.Scopes, spec.Middlewares)
		}
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
// Supports graceful shutdown on 'kill' and 'int' signals.
func (app *App) Run() error {
	if app.vulcandReg != nil {
		err := app.vulcandReg.Start()
		if err != nil {
			return fmt.Errorf("failed to start vulcand registry: err=(%s)", err)
		}
		heartbeatCh := make(chan os.Signal, 1)
		signal.Notify(heartbeatCh, syscall.SIGUSR1)
		go func() {
			sig := <-heartbeatCh
			log.Infof("Got signal %v, canceling vulcand registration", sig)
			app.vulcandReg.Stop()
		}()
	}

	// listen for a shutdown signal
	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		s := <-exitCh
		log.Infof("Got signal %v, shutting down", s)
		if app.vulcandReg != nil {
			app.vulcandReg.Stop()
		}
		manners.Close()
	}()

	addr := fmt.Sprintf("%v:%v", app.Config.ListenIP, app.Config.ListenPort)
	return manners.ListenAndServe(addr, app.router)
}

// registerLocation is a helper for registering handlers in vulcan.
func (app *App) registerFrontend(methods []string, path string, scopes []Scope, middlewares []vulcand.Middleware) error {
	for _, scope := range scopes {
		host, err := app.apiHostForScope(scope)
		if err != nil {
			return err
		}
		app.vulcandReg.AddFrontend(host, path, methods, middlewares)
	}
	return nil
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

func handlePing(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}
