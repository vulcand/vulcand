package supervisor

import (
	"fmt"
	"sync"
	"time"

	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	"github.com/mailgun/vulcand/connwatch"

	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/server"
)

// Supervisor watches changes to the dynamic backends and applies those changes to the server in real time.
// Supervisor handles lifetime of the proxy as well, and does graceful restarts and recoveries in case of failures.
type Supervisor struct {
	// lastId allows to create iterative server instance versions for debugging purposes.
	lastId int

	// wg allows to wait for graceful shutdowns
	wg *sync.WaitGroup

	mtx *sync.RWMutex

	// srv is the current active server
	srv server.Server
	// newBackend function creates backend clients when called
	newBackend backend.NewBackendFn

	// newSrv returns new server instance every time is called.
	newSrv server.NewServerFn

	// timeProvider is used to mock time in tests
	timeProvider timetools.TimeProvider

	// errorC is a channel will be used to notify the calling party of the errors.
	errorC chan error
	// restartC channel is used internally to trigger graceful restarts on errors and configuration changes.
	restartC chan error
	// closeC is a channel to tell everyone to stop working and exit at the earliest convenience.
	closeC chan bool

	connWatcher *connwatch.ConnectionWatcher

	options Options

	started bool
}

type Options struct {
	TimeProvider timetools.TimeProvider
}

func NewSupervisor(newSrv server.NewServerFn, newBackend backend.NewBackendFn, errorC chan error) (s *Supervisor) {
	return NewSupervisorWithOptions(newSrv, newBackend, errorC, Options{})
}

func NewSupervisorWithOptions(newSrv server.NewServerFn, newBackend backend.NewBackendFn, errorC chan error, options Options) (s *Supervisor) {
	return &Supervisor{
		wg:          &sync.WaitGroup{},
		mtx:         &sync.RWMutex{},
		newSrv:      newSrv,
		newBackend:  newBackend,
		options:     parseOptions(options),
		errorC:      errorC,
		restartC:    make(chan error),
		closeC:      make(chan bool),
		connWatcher: connwatch.NewConnectionWatcher(),
	}
}

func (s *Supervisor) getCurrentServer() server.Server {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.srv
}

func (s *Supervisor) setCurrentServer(srv server.Server) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.srv = srv
}

func (s *Supervisor) isStarted() bool {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.started
}

func (s *Supervisor) setStarted() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.started = true
}

func (s *Supervisor) GetConnWatcher() *connwatch.ConnectionWatcher {
	return s.connWatcher
}

func (s *Supervisor) GetStats(hostname, locationId string, e *backend.Endpoint) *backend.EndpointStats {
	srv := s.getCurrentServer()
	if srv != nil {
		return srv.GetStats(hostname, locationId, e)
	}
	return nil
}

func (s *Supervisor) init() error {
	srv, err := s.newSrv(s.lastId, s.connWatcher)
	if err != nil {
		return err
	}
	s.lastId += 1

	backend, err := s.newBackend()
	if err != nil {
		return err
	}

	if err := initServer(backend, srv); err != nil {
		return err
	}

	log.Infof("%s init() initial setup done", srv)

	oldSrv := s.getCurrentServer()
	if oldSrv != nil {
		if err := srv.HijackListenersFrom(oldSrv); err != nil {
			return err
		}
	}

	if err := srv.Start(); err != nil {
		return err
	}

	if oldSrv != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			oldSrv.Stop(true)
		}()
	}

	// Watch and configure this instance of server
	s.setCurrentServer(srv)
	changesC := make(chan interface{})

	// This goroutine will connect to the backend and emit the changes to the changesC channel.
	// In case of any error it notifies supervisor of the error by sending an error to the channel triggering reload.
	go func() {
		if err := backend.WatchChanges(changesC); err != nil {
			log.Infof("%s backend watcher got error: '%s' will restart", srv, err)
			backend.Close()
			close(changesC)
			s.restartC <- err
		} else {
			// Graceful shutdown without restart
			backend.Close()
			log.Infof("%s backend watcher got nil error, gracefully shutdown", srv)
			s.restartC <- nil
		}
	}()

	// This goroutine will listen for changes arriving to the changes channel and reconfigure the given server
	go func() {
		for {
			change := <-changesC
			if change == nil {
				log.Infof("Stop watching changes for %s", srv)
				return
			}
			if err := processChange(srv, change); err != nil {
				log.Errorf("Failed to process change %#v, err: %s", change, err)
			}
		}
	}()
	return nil
}

func (s *Supervisor) stop() {
	srv := s.getCurrentServer()
	if srv != nil {
		srv.Stop(true)
		log.Infof("%s was stopped by supervisor", srv)
	}
	log.Infof("Wait for any outstanding operations to complete")
	s.wg.Wait()
	log.Infof("All outstanding operations have been completed, signalling stop")
	close(s.closeC)
}

// supervise() listens for error notifications and triggers graceful restart
func (s *Supervisor) supervise() {
	for {
		err := <-s.restartC

		// This means graceful shutdown, do nothing and return
		if err == nil {
			log.Infof("watchErrors - graceful shutdown")
			s.stop()
			return
		}
		for {
			s.options.TimeProvider.Sleep(retryPeriod)
			log.Infof("supervise() restarting %s on error: %s", s.srv, err)
			// We failed to initialize server, this error can not be recovered, so send an error and exit
			if err := s.init(); err != nil {
				log.Infof("Failed to initialize %s, will retry", err)
			} else {
				break
			}
		}
	}
}

func (s *Supervisor) Start() error {
	defer s.setStarted()
	go s.supervise()
	return s.init()
}

func (s *Supervisor) Stop(wait bool) {
	close(s.restartC)
	if !s.isStarted() {
		return
	}
	if wait {
		<-s.closeC
		log.Infof("All operations stopped")
	}
}

// initServer reads the configuration from the backend and configures the server
func initServer(backend backend.Backend, srv server.Server) error {
	hosts, err := backend.GetHosts()
	if err != nil {
		log.Infof("Error getting hosts: %s", err)
		return err
	}

	if len(hosts) == 0 {
		log.Warningf("No hosts found")
	}

	for _, h := range hosts {
		if err := srv.UpsertHost(h); err != nil {
			return err
		}
		for _, l := range h.Locations {
			if err := srv.UpsertLocation(h, l); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseOptions(o Options) Options {
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	return o
}

// processChange takes the backend change notification emitted by the backend and applies it to the server
func processChange(s server.Server, ch interface{}) error {
	switch change := ch.(type) {
	case *backend.HostAdded:
		return s.UpsertHost(change.Host)
	case *backend.HostDeleted:
		return s.DeleteHost(change.Name)
	case *backend.HostCertUpdated:
		return s.UpdateHostCert(change.Host.Name, change.Host.Cert)
	case *backend.HostListenerAdded:
		return s.AddHostListener(change.Host, change.Listener)
	case *backend.HostListenerDeleted:
		return s.DeleteHostListener(change.Host, change.ListenerId)
	case *backend.LocationAdded:
		return s.UpsertLocation(change.Host, change.Location)
	case *backend.LocationDeleted:
		return s.DeleteLocation(change.Host, change.LocationId)
	case *backend.LocationUpstreamUpdated:
		return s.UpdateLocationUpstream(change.Host, change.Location)
	case *backend.LocationPathUpdated:
		return s.UpdateLocationPath(change.Host, change.Location, change.Path)
	case *backend.LocationOptionsUpdated:
		return s.UpdateLocationOptions(change.Host, change.Location)
	case *backend.LocationMiddlewareAdded:
		return s.UpsertLocationMiddleware(change.Host, change.Location, change.Middleware)
	case *backend.LocationMiddlewareUpdated:
		return s.UpsertLocationMiddleware(change.Host, change.Location, change.Middleware)
	case *backend.LocationMiddlewareDeleted:
		return s.DeleteLocationMiddleware(change.Host, change.Location, change.MiddlewareType, change.MiddlewareId)
	case *backend.UpstreamAdded:
		return nil
	case *backend.UpstreamDeleted:
		return nil
	case *backend.EndpointAdded:
		return s.UpsertEndpoint(change.Upstream, change.Endpoint, change.AffectedLocations)
	case *backend.EndpointUpdated:
		return s.UpsertEndpoint(change.Upstream, change.Endpoint, change.AffectedLocations)
	case *backend.EndpointDeleted:
		return s.DeleteEndpoint(change.Upstream, change.EndpointId, change.AffectedLocations)
	}
	return fmt.Errorf("unsupported change: %#v", ch)
}

const retryPeriod = 5 * time.Second
