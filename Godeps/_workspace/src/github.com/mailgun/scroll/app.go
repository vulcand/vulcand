package scroll

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/mailgun/log"
	"github.com/mailgun/manners"
	"github.com/mailgun/metrics"
)

const (
	// Suggested result set limit for APIs that may return many entries (e.g. paging).
	DefaultLimit int = 100

	// Suggested max allowed result set limit for APIs that may return many entries (e.g. paging).
	MaxLimit int = 10000

	// Suggested max allowed amount of entries that batch APIs can accept (e.g. batch uploads).
	MaxBatchSize int = 1000
)

// Represents an app.
type App struct {
	config   AppConfig
	router   *mux.Router
	registry *registry
	stats    *appStats
}

// Represents a configuration object an app is created with.
type AppConfig struct {
	// name of the app being created
	Name string

	// host the app is intended to bind to
	Host string

	// port the app is going to listen on
	Port int

	// optional router to use
	Router *mux.Router

	// hostname of the public API entrypoint used for vulcand registration
	APIHost string

	// whether to register the app's endpoint and handlers in vulcand
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
	var registry *registry
	if config.Register != false {
		registry = newRegistry()
	}

	router := config.Router
	if router == nil {
		router = mux.NewRouter()
	}

	return &App{
		config:   config,
		router:   router,
		registry: registry,
		stats:    newAppStats(config.Client),
	}
}

// Register a handler function.
//
// If vulcand registration is enabled in the both app config and handler spec,
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

	// vulcand registration
	if app.registry != nil && spec.Register != false {
		app.registerLocation(spec.Methods, spec.Path)
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

// Start the app on the configured host/port.
//
// If vulcand registration is enabled in the app config, starts a goroutine that
// will be registering the app's endpoint once every minute in the local etcd
// instance.
//
// Supports graceful shutdown on 'kill' and 'int' signals.
func (app *App) Run() error {
	http.Handle("/", app.router)

	if app.registry != nil {
		go func() {
			for {
				app.registerEndpoint()
				time.Sleep(60 * time.Second)
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

	return manners.ListenAndServe(fmt.Sprintf("%v:%v", app.config.Host, app.config.Port), nil)
}

// Helper function to register the app's endpoint in vulcand.
func (app *App) registerEndpoint() error {
	endpoint := newEndpoint(app.config.Name, app.config.Host, app.config.Port)

	if err := app.registry.RegisterEndpoint(endpoint); err != nil {
		return err
	}

	log.Infof("Registered endpoint: %v", endpoint)

	return nil
}

// Helper function to register handlers in vulcand.
func (app *App) registerLocation(methods []string, path string) error {
	location := newLocation(app.config.APIHost, methods, path, app.config.Name)

	if err := app.registry.RegisterLocation(location); err != nil {
		return err
	}

	log.Infof("Registered location: %v", location)

	return nil
}
