package supervisor

import (
	"fmt"
	"sync"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"

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

	// newSrv returns new server instance every time is called.
	newSrv server.NewServerFn

	// timeProvider is used to mock time in tests
	timeProvider timetools.TimeProvider

	// backend is used for reading initial configuration
	backend backend.Backend

	// errorC is a channel will be used to notify the calling party of the errors.
	errorC chan error
	// restartC channel is used internally to trigger graceful restarts on errors and configuration changes.
	restartC chan error
	// closeC is a channel to tell everyone to stop working and exit at the earliest convenience.
	closeC chan bool

	options Options

	state supervisorState
}

type Options struct {
	TimeProvider timetools.TimeProvider
	Files        []*server.FileDescriptor
}

func NewSupervisor(newSrv server.NewServerFn, backend backend.Backend, errorC chan error) (s *Supervisor) {
	return NewSupervisorWithOptions(newSrv, backend, errorC, Options{})
}

func NewSupervisorWithOptions(newSrv server.NewServerFn, backend backend.Backend, errorC chan error, options Options) (s *Supervisor) {
	return &Supervisor{
		wg:       &sync.WaitGroup{},
		mtx:      &sync.RWMutex{},
		newSrv:   newSrv,
		backend:  backend,
		options:  parseOptions(options),
		errorC:   errorC,
		restartC: make(chan error),
		closeC:   make(chan bool),
	}
}

func (s *Supervisor) String() string {
	return fmt.Sprintf("Supervisor(%v)", s.state)
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

func (s *Supervisor) GetFiles() ([]*server.FileDescriptor, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.srv != nil {
		return s.srv.GetFiles()
	}
	return []*server.FileDescriptor{}, nil
}

func (s *Supervisor) setState(state supervisorState) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.state = state
}

func (s *Supervisor) GetLocationStats(l *backend.Location) (*backend.RoundTripStats, error) {
	srv := s.getCurrentServer()
	if srv != nil {
		return srv.GetLocationStats(l)
	}
	return nil, fmt.Errorf("no current server")
}

func (s *Supervisor) GetEndpointStats(e *backend.Endpoint) (*backend.RoundTripStats, error) {
	srv := s.getCurrentServer()
	if srv != nil {
		return srv.GetEndpointStats(e)
	}
	return nil, fmt.Errorf("no current server")
}

func (s *Supervisor) GetUpstreamStats(u *backend.Upstream) (*backend.RoundTripStats, error) {
	srv := s.getCurrentServer()
	if srv != nil {
		return srv.GetUpstreamStats(u)
	}
	return nil, fmt.Errorf("no current server")
}

func (s *Supervisor) GetTopLocations(hostname, upstreamId string) ([]*backend.Location, error) {
	srv := s.getCurrentServer()
	if srv != nil {
		return srv.GetTopLocations(hostname, upstreamId)
	}
	return nil, fmt.Errorf("no current server")
}

func (s *Supervisor) GetTopEndpoints(upstreamId string) ([]*backend.Endpoint, error) {
	srv := s.getCurrentServer()
	if srv != nil {
		return srv.GetTopEndpoints(upstreamId)
	}
	return nil, fmt.Errorf("no current server")
}

func (s *Supervisor) init() error {
	srv, err := s.newSrv(s.lastId)
	if err != nil {
		return err
	}
	s.lastId += 1

	if err := initServer(s.backend, srv); err != nil {
		return err
	}

	// This is the first start, pass the files that could have been passed
	// to us by the parent process
	if s.lastId == 1 && len(s.options.Files) != 0 {
		log.Infof("Passing files %s to %s", s.options.Files, srv)
		if err := srv.TakeFiles(s.options.Files); err != nil {
			return err
		}
	}

	log.Infof("%s init() initial setup done", srv)

	oldSrv := s.getCurrentServer()
	if oldSrv != nil {
		files, err := oldSrv.GetFiles()
		if err != nil {
			return err
		}
		if err := srv.TakeFiles(files); err != nil {
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
		cancelC := make(chan bool)
		if err := s.backend.WatchChanges(changesC, cancelC); err != nil {
			log.Infof("%s backend watcher got error: '%s' will restart", srv, err)
			close(cancelC)
			close(changesC)
			s.restartC <- err
		} else {
			close(cancelC)
			// Graceful shutdown without restart
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
				log.Errorf("failed to process change %#v, err: %s", change, err)
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
	if s.checkAndSetState(supervisorStateActive) {
		return fmt.Errorf("%v already started", s)
	}
	defer s.setState(supervisorStateActive)
	go s.supervise()
	return s.init()
}

func (s *Supervisor) checkAndSetState(state supervisorState) bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if s.state == state {
		return true
	}

	s.state = state
	return false
}

func (s *Supervisor) Stop(wait bool) {

	// It was already stopped
	if s.checkAndSetState(supervisorStateStopped) {
		return
	}

	close(s.restartC)
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
	case *backend.HostKeyPairUpdated:
		return s.UpdateHostKeyPair(change.Host.Name, change.Host.KeyPair)
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
		return s.AddUpstream(change.Upstream)
	case *backend.UpstreamDeleted:
		return s.DeleteUpstream(change.UpstreamId)
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

type supervisorState int

const (
	supervisorStateCreated = iota
	supervisorStateActive
	supervisorStateStopped
)

func (s supervisorState) String() string {
	switch s {
	case supervisorStateCreated:
		return "created"
	case supervisorStateActive:
		return "active"
	case supervisorStateStopped:
		return "stopped"
	default:
		return "unkown"
	}
}
