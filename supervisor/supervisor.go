package supervisor

import (
	"fmt"
	"sync"
	"time"

	"github.com/mailgun/timetools"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/proxy"
)

const (
	retryPeriod       = 5 * time.Second
	changesBufferSize = 2000
)

// Supervisor watches changes to the dynamic backends and applies those changes
// to the server in real time. Supervisor handles lifetime of the proxy as well,
// and does graceful restarts and recoveries in case of failures.
type Supervisor struct {
	// lastId allows to create iterative server instance versions for debugging
	// purposes.
	lastId int

	options Options

	mtx sync.RWMutex

	// srv is the current active server
	proxy proxy.Proxy

	// newProxyFn returns new mux instance every time is called.
	newProxyFn proxy.NewProxyFn

	// timeProvider is used to mock time in tests
	timeProvider timetools.TimeProvider

	// engine is used for reading configuration details
	engine engine.Engine

	watcherWg      sync.WaitGroup
	watcherCancelC chan struct{}
	watcherErrorC  chan struct{}

	stopWg sync.WaitGroup
	stopC  chan struct{}
}

type Options struct {
	Clock timetools.TimeProvider
	Files []*proxy.FileDescriptor
}

func New(newProxy proxy.NewProxyFn, engine engine.Engine, options Options) *Supervisor {
	return &Supervisor{
		newProxyFn: newProxy,
		engine:     engine,
		options:    setDefaults(options),
		stopC:      make(chan struct{}),
	}
}

func (s *Supervisor) Start() error {
	if err := s.init(); err != nil {
		return errors.Wrap(err, "initialization failed")
	}
	s.stopWg.Add(1)
	go s.run()
	return nil
}

func (s *Supervisor) Stop() {
	close(s.stopC)
	s.stopWg.Wait()
	log.Infof("All operations stopped")
}

func (s *Supervisor) String() string {
	return "sup"
}

func (s *Supervisor) GetFiles() ([]*proxy.FileDescriptor, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.proxy != nil {
		return s.proxy.GetFiles()
	}
	return []*proxy.FileDescriptor{}, nil
}

func (s *Supervisor) FrontendStats(key engine.FrontendKey) (*engine.RoundTripStats, error) {
	p := s.getCurrentProxy()
	if p != nil {
		return p.FrontendStats(key)
	}
	return nil, fmt.Errorf("no current proxy")
}

func (s *Supervisor) ServerStats(key engine.ServerKey) (*engine.RoundTripStats, error) {
	p := s.getCurrentProxy()
	if p != nil {
		return p.ServerStats(key)
	}
	return nil, fmt.Errorf("no current proxy")
}

func (s *Supervisor) BackendStats(key engine.BackendKey) (*engine.RoundTripStats, error) {
	p := s.getCurrentProxy()
	if p != nil {
		return p.BackendStats(key)
	}
	return nil, fmt.Errorf("no current proxy")
}

// TopFrontends returns locations sorted by criteria (faulty, slow, most used)
// if hostname or backendId is present, will filter out locations for that host
// or backendId.
func (s *Supervisor) TopFrontends(key *engine.BackendKey) ([]engine.Frontend, error) {
	p := s.getCurrentProxy()
	if p != nil {
		return p.TopFrontends(key)
	}
	return nil, fmt.Errorf("no current proxy")
}

// TopServers returns endpoints sorted by criteria (faulty, slow, mos used)
// if backendId is not empty, will filter out endpoints for that backendId.
func (s *Supervisor) TopServers(key *engine.BackendKey) ([]engine.Server, error) {
	p := s.getCurrentProxy()
	if p != nil {
		return p.TopServers(key)
	}
	return nil, fmt.Errorf("no current proxy")
}

func (s *Supervisor) getCurrentProxy() proxy.Proxy {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.proxy
}

func (s *Supervisor) setCurrentProxy(p proxy.Proxy) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.proxy = p
}

func (s *Supervisor) init() error {
	snapshot, err := s.engine.GetSnapshot()
	if err != nil {
		return errors.Wrap(err, "failed to get snapshot")
	}

	newMuxId := s.lastId
	s.lastId += 1

	// Subscribe for updates right away so we can get beyond 1000 updates hard
	// coded Etcd limit. This way we will be able to survive handle larger
	// update bursts that happen while multiplexer is being initialized and
	// therefore does not handle updates.
	cancelWatcher := true
	changesC := make(chan interface{}, changesBufferSize)
	s.watcherErrorC = make(chan struct{}, 1)
	s.watcherCancelC = make(chan struct{})
	s.watcherWg.Add(1)
	go func() {
		defer s.watcherWg.Done()
		defer close(changesC)
		if err := s.engine.Subscribe(changesC, snapshot.Index, s.watcherCancelC); err != nil {
			log.Infof("mux_%d engine watcher failed: '%v' will restart", newMuxId, err)
			s.watcherErrorC <- struct{}{}
			return
		}
		log.Infof("nux_%d engine watcher shutdown", newMuxId)
	}()
	// Make sure watcher goroutine is stopped if initialization fails.
	defer func() {
		if cancelWatcher {
			close(s.watcherCancelC)
			s.watcherWg.Wait()
		}
	}()

	checkpoint := time.Now()
	newProxy, err := s.newProxyFn(newMuxId)
	if err != nil {
		return errors.Wrap(err, "failed to create mux")
	}
	if err = newProxy.Init(*snapshot); err != nil {
		return errors.Wrap(err, "failed to init mux")
	}
	log.Infof("%v initial setup done, took=%v", newProxy, time.Now().Sub(checkpoint))

	// If it is initialization on process sturtup then take over files from the
	// parrent process if any.
	if s.lastId == 1 && len(s.options.Files) != 0 {
		log.Infof("Passing files %v to %v", s.options.Files, newProxy)
		if err := newProxy.TakeFiles(s.options.Files); err != nil {
			return errors.Wrap(err, "failed to inherit files from parrent process")
		}
	}

	// If it is recovery from the previous multiplexer failure then take over
	// files from the failed multiplexer.
	oldProxy := s.getCurrentProxy()
	if oldProxy != nil {
		log.Infof("%v taking files from %v to %v", s, oldProxy, newProxy)
		files, err := oldProxy.GetFiles()
		if err != nil {
			return errors.Wrapf(err, "cannot get file list from %v", oldProxy)
		}
		if err := newProxy.TakeFiles(files); err != nil {
			return errors.Wrapf(err, "failed to take files from %v", oldProxy)
		}
	}

	if err := newProxy.Start(); err != nil {
		return errors.Wrapf(err, "failed to start new mux %v", newProxy)
	}
	s.setCurrentProxy(newProxy)
	// A new multiplexer has been successfully started therefore we do not need
	// to cancel the watcher, the supervisor run thread will take care of it.
	cancelWatcher = false

	// Shutdown the old mux on the background.
	if oldProxy != nil {
		s.stopWg.Add(1)
		go func() {
			defer s.stopWg.Done()
			checkpoint := time.Now()
			oldProxy.Stop(true)
			log.Infof("%v old mux stopped %v, took=%v", s, oldProxy, time.Now().Sub(checkpoint))
		}()
	}

	// This goroutine will listen for changes arriving to the changes channel
	// and reconfigure the given server.
	s.watcherWg.Add(1)
	go func() {
		defer s.watcherWg.Done()
		for change := range changesC {
			if err := processChange(newProxy, change); err != nil {
				log.Errorf("%v failed to process, change=%#v, err=%s", newProxy, change, err)
			}
		}
		log.Infof("%v change processor shutdown", newProxy)
	}()
	return nil
}

// supervise listens for error notifications and triggers graceful restart.
func (s *Supervisor) run() {
	defer s.stopWg.Done()
	for {
		select {
		case <-s.watcherErrorC:
			s.watcherWg.Wait()
			s.watcherErrorC = nil
		case <-s.stopC:
			close(s.watcherCancelC)
			s.engine.Close()
			s.watcherWg.Wait()
			if s.proxy != nil {
				s.proxy.Stop(true)
			}
			return
		}

		err := s.init()
		// In case of an error keep trying to initialize making pauses between
		// attempts.
		for err != nil {
			log.Errorf("sup failed to reinit, err=%v", err)
			select {
			case <-time.After(retryPeriod):
			case <-s.stopC:
				return
			}
			err = s.init()
		}
	}
}

func setDefaults(o Options) Options {
	if o.Clock == nil {
		o.Clock = &timetools.RealTime{}
	}
	return o
}

// processChange takes the backend change notification emitted by the backend
// and applies it to the server.
func processChange(p proxy.Proxy, ch interface{}) error {
	switch change := ch.(type) {
	case *engine.HostUpserted:
		return p.UpsertHost(change.Host)
	case *engine.HostDeleted:
		return p.DeleteHost(change.HostKey)

	case *engine.ListenerUpserted:
		return p.UpsertListener(change.Listener)

	case *engine.ListenerDeleted:
		return p.DeleteListener(change.ListenerKey)

	case *engine.FrontendUpserted:
		return p.UpsertFrontend(change.Frontend)
	case *engine.FrontendDeleted:
		return p.DeleteFrontend(change.FrontendKey)

	case *engine.MiddlewareUpserted:
		return p.UpsertMiddleware(change.FrontendKey, change.Middleware)

	case *engine.MiddlewareDeleted:
		return p.DeleteMiddleware(change.MiddlewareKey)

	case *engine.BackendUpserted:
		return p.UpsertBackend(change.Backend)
	case *engine.BackendDeleted:
		return p.DeleteBackend(change.BackendKey)

	case *engine.ServerUpserted:
		return p.UpsertServer(change.BackendKey, change.Server)
	case *engine.ServerDeleted:
		return p.DeleteServer(change.ServerKey)
	}
	return fmt.Errorf("unsupported change: %#v", ch)
}
