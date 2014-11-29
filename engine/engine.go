package engine

import (
	"time"

	"github.com/mailgun/vulcand/plugin"
)

type NewEngineFn func() (Engine, error)

// Engine is an interface for storage and configuration engine, e.g. Etcd.
type Engine interface {
	GetHosts() ([]Host, error)
	GetHost(HostKey) (*Host, error)
	UpsertHost(Host) error
	DeleteHost(HostKey) error

	GetListeners(HostKey) ([]Listener, error)
	GetListener(ListenerKey) (*Listener, error)
	UpsertListener(HostKey, Listener) error
	DeleteListener(ListenerKey) error

	GetFrontends() ([]Frontend, error)
	GetFrontend(FrontendKey) (*Frontend, error)
	UpsertFrontend(Frontend, time.Duration) error
	DeleteFrontend(FrontendKey) error

	GetMiddlewares(FrontendKey) ([]Middleware, error)
	GetMiddleware(MiddlewareKey) (*Middleware, error)
	UpsertMiddleware(FrontendKey, Middleware, time.Duration) error
	DeleteMiddleware(MiddlewareKey) error

	GetBackends() ([]Backend, error)
	GetBackend(BackendKey) (*Backend, error)
	UpsertBackend(Backend) error
	DeleteBackend(BackendKey) error

	GetServers(BackendKey) ([]Server, error)
	GetServer(ServerKey) (*Server, error)
	UpsertServer(BackendKey, Server, time.Duration) error
	DeleteServer(ServerKey) error

	// Subscribe is an entry point for getting the configuration changes as well as the initial configuration.
	// It should be a blocking function generating events from change.go to the changes channel.
	Subscribe(events chan interface{}, cancel chan bool) error

	// GetRegistry returns registry with the supported plugins.
	GetRegistry() *plugin.Registry

	// Close
	Close()
}
