package proxy

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/mailgun/metrics"
	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/vulcand/conntracker"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/plugin"
	"github.com/vulcand/vulcand/plugin/cacheprovider"
	"github.com/vulcand/vulcand/router"
)

type HealthCheckOptions struct {
	// The interval at which vulcand will run health checks on the backend servers
	Interval time.Duration
	// The path that will be used if no HealthCheckPath is provided by the backend
	// If no HealthCheckPath provided, HealthCheck is disabled
	HealthCheckPath string
	// Timeout is how long the health check should wait for a response
	Timeout time.Duration
	// UnHealthyBackendDuration specifies how long we should wait until it marks the
	// backend as failed, if all the backend servers report unhealthy.
	UnHealthyBackendDuration time.Duration
}

type Proxy interface {
	engine.StatsProvider

	Init(snapshot engine.Snapshot) error

	UpsertHost(engine.Host) error
	DeleteHost(engine.HostKey) error

	UpsertListener(engine.Listener) error
	DeleteListener(engine.ListenerKey) error

	UpsertBackend(engine.Backend) error
	DeleteBackend(engine.BackendKey) error

	UpsertFrontend(engine.Frontend) error
	DeleteFrontend(engine.FrontendKey) error

	UpsertMiddleware(engine.FrontendKey, engine.Middleware) error
	DeleteMiddleware(engine.MiddlewareKey) error

	UpsertServer(engine.BackendKey, engine.Server) error
	DeleteServer(engine.ServerKey) error

	// TakeFiles takes file descriptors representing sockets in listening state to start serving on them
	// instead of binding. This is nessesary if the child process needs to inherit sockets from the parent
	// (e.g. for graceful restarts)
	TakeFiles([]*FileDescriptor) error

	// GetFiles exports listening socket's underlying dupped file descriptors, so they can later
	// be passed to child process or to another Server
	GetFiles() ([]*FileDescriptor, error)

	Start() error
	Stop(wait bool)

	// HealthCheckServers will monitor the backend servers by calling health checks on them
	// if all backend servers are unhealthy will return with an error.
	HealthCheckServers(done chan struct{}, opts HealthCheckOptions) error
}

type Options struct {
	MetricsClient             metrics.Client
	DialTimeout               time.Duration
	ReadTimeout               time.Duration
	WriteTimeout              time.Duration
	MaxHeaderBytes            int
	DefaultListener           *engine.Listener
	TrustForwardHeader        bool
	Files                     []*FileDescriptor
	TimeProvider              timetools.TimeProvider
	NotFoundMiddleware        plugin.Middleware
	Router                    router.Router
	IncomingConnectionTracker conntracker.ConnectionTracker
	FrontendListeners         plugin.FrontendListeners
	CacheProvider             cacheprovider.T
	Aliases                   map[string]string
}

type NewProxyFn func(id int) (Proxy, error)

type FileDescriptor struct {
	Address engine.Address
	File    *os.File
}

func (fd *FileDescriptor) ToListener() (net.Listener, error) {
	listener, err := net.FileListener(fd.File)
	if err != nil {
		return nil, err
	}
	fd.File.Close()
	return listener, nil
}

func (fd *FileDescriptor) String() string {
	return fmt.Sprintf("FileDescriptor(%s, %d)", fd.Address, fd.File.Fd())
}

// DefaultNotFound is an HTTP handler that returns simple 404 Not Found response.
var DefaultNotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	log.Infof("Not found: %v %v", r.Method, r.URL)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, `{"error":"not found"}`)
})
