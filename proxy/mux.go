package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/mailgun/metrics"
	"github.com/mailgun/timetools"
	"github.com/pkg/errors"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/route"
	"github.com/vulcand/vulcand/conntracker"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/router"
	"github.com/vulcand/vulcand/stapler"
)

// mux is capable of listening on multiple interfaces, graceful shutdowns and updating TLS certificates
type mux struct {
	// Debugging id
	id int

	// Each listener address has a server associated with it
	servers map[engine.ListenerKey]*srv

	backends map[engine.BackendKey]backendEntry

	frontends map[engine.FrontendKey]*frontend

	hosts map[engine.HostKey]engine.Host

	// Options hold parameters that are used to initialize http servers
	options Options

	// Wait group for graceful shutdown
	wg sync.WaitGroup

	// Read write mutex for serialized operations
	mtx sync.RWMutex

	// Router will be shared between multiple listeners
	router router.Router

	// Current server stats
	state muxState

	// Connection watcher
	incomingConnTracker conntracker.ConnectionTracker

	// Connection watcher
	outgoingConnTracker forward.UrlForwardingStateListener

	// stopC used for global broadcast to all proxy systems that it's closed
	stopC chan struct{}

	// OCSP staple cache and responder
	stapler stapler.Stapler

	// Unsubscribe from staple updates
	stapleUpdatesC chan *stapler.StapleUpdated
}

type backendEntry struct {
	backend   *backend
	frontends map[engine.FrontendKey]*frontend
}

func newBackendEntry(beCfg engine.Backend, opts Options, beSrvCfgs []engine.Server) (backendEntry, error) {
	be, err := newBackend(beCfg, opts, beSrvCfgs)
	if err != nil {
		return backendEntry{}, errors.Wrap(err, "failed to create backend")
	}
	return backendEntry{
		backend:   be,
		frontends: make(map[engine.FrontendKey]*frontend),
	}, nil
}

func (m *mux) String() string {
	return fmt.Sprintf("mux_%d", m.id)
}

func New(id int, st stapler.Stapler, o Options) (*mux, error) {
	o = setDefaults(o)
	m := &mux{
		id:      id,
		options: o,

		router:              o.Router,
		incomingConnTracker: o.IncomingConnectionTracker,
		outgoingConnTracker: o.OutgoingConnectionTracker,

		servers:   make(map[engine.ListenerKey]*srv),
		backends:  make(map[engine.BackendKey]backendEntry),
		frontends: make(map[engine.FrontendKey]*frontend),
		hosts:     make(map[engine.HostKey]engine.Host),

		stapleUpdatesC: make(chan *stapler.StapleUpdated),
		stopC:          make(chan struct{}),
		stapler:        st,
	}

	m.router.SetNotFound(&DefaultNotFound{})
	if o.NotFoundMiddleware != nil {
		if handler, err := o.NotFoundMiddleware.NewHandler(m.router.GetNotFound()); err == nil {
			m.router.SetNotFound(handler)
		}
	}

	if m.options.DefaultListener != nil {
		if err := m.upsertListener(*m.options.DefaultListener); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *mux) Init(ss engine.Snapshot) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, host := range ss.Hosts {
		m.hosts[engine.HostKey{Name: host.Name}] = host
	}

	for _, bes := range ss.BackendSpecs {
		beKey := engine.BackendKey{Id: bes.Backend.Id}
		srvCfgs := make([]engine.Server, len(bes.Servers))
		for i, beSrvCfg := range bes.Servers {
			if _, err := url.ParseRequestURI(beSrvCfg.URL); err != nil {
				return errors.Wrapf(err, "failed to parse %v", beSrvCfg)
			}
			srvCfgs[i] = beSrvCfg
		}
		beEnt, err := newBackendEntry(bes.Backend, m.options, srvCfgs)
		if err != nil {
			return errors.Wrapf(err, "failed to create backend entry %v", bes.Backend.Id)
		}
		m.backends[beKey] = beEnt
	}

	for _, l := range ss.Listeners {
		for _, feSrv := range m.servers {
			if feSrv.listener.Address == l.Address {
				// This only exists to simplify test fixture configuration.
				if feSrv.listener.Id == l.Id {
					continue
				}
				return errors.Errorf("%v conflicts with existing %v", l.Id, feSrv.listener.Id)
			}
		}
		feSrv, err := newSrv(m, l)
		if err != nil {
			return errors.Wrapf(err, "failed to create server %v", l.Id)
		}
		m.servers[engine.ListenerKey{Id: l.Id}] = feSrv
	}

	for _, fes := range ss.FrontendSpecs {
		feKey := engine.FrontendKey{fes.Frontend.Id}
		beEnt, ok := m.backends[engine.BackendKey{Id: fes.Frontend.BackendId}]
		if !ok {
			return errors.Errorf("unknown backend %v in frontend %v",
				fes.Frontend.BackendId, fes.Frontend.Id)
		}
		mwCfgs := make(map[engine.MiddlewareKey]engine.Middleware)
		for _, mw := range fes.Middlewares {
			mwCfgs[engine.MiddlewareKey{FrontendKey: feKey, Id: mw.Id}] = mw
		}
		fe := newFrontend(fes.Frontend, beEnt.backend, m.options, mwCfgs, m.outgoingConnTracker)
		if err := m.router.Handle(fes.Frontend.Route, fe); err != nil {
			return errors.Wrapf(err, "cannot add route %v for frontend %v",
				fes.Frontend.Route, fes.Frontend.Id)
		}
		m.frontends[feKey] = fe
		beEnt.frontends[feKey] = fe
	}
	return nil
}

func (m *mux) GetFiles() ([]*FileDescriptor, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	fds := []*FileDescriptor{}

	for _, feSrv := range m.servers {
		fd, err := feSrv.GetFile()
		if err != nil {
			return nil, err
		}
		if fd != nil {
			fds = append(fds, fd)
		}
	}
	return fds, nil
}

func (m *mux) TakeFiles(files []*FileDescriptor) error {
	log.Infof("%s TakeFiles %s", m, files)

	fMap := make(map[engine.Address]*FileDescriptor, len(files))
	for _, f := range files {
		fMap[f.Address] = f
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, srv := range m.servers {

		file, exists := fMap[srv.listener.Address]
		if !exists {
			log.Infof("%s skipping take of files from address %s, has no passed files", m, srv.listener.Address)
			continue
		}
		if err := srv.takeFile(file); err != nil {
			return err
		}
	}

	return nil
}

func (m *mux) Start() error {
	log.Infof("%s start", m)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.state != stateInit {
		return fmt.Errorf("%s can start only from init state, got %d", m, m.state)
	}

	// Subscribe to staple responses and kick staple updates
	m.stapler.Subscribe(m.stapleUpdatesC, m.stopC)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.stopC:
				log.Infof("%v stop listening for staple updates", m)
				return
			case e := <-m.stapleUpdatesC:
				m.processStapleUpdate(e)
			}
		}
	}()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.stopC:
				log.Infof("%v stop emitting metrics", m)
				return
			case <-time.After(time.Second):
				if err := m.emitMetrics(); err != nil {
					log.Errorf("%v failed to emit metrics, err=%v", m, err)
				}
			}
		}
	}()

	m.state = stateActive
	for _, s := range m.servers {
		if err := s.start(); err != nil {
			return err
		}
	}

	log.Infof("%s started", m)
	return nil
}

func (m *mux) Stop(wait bool) {
	log.Infof("%s Stop(%t)", m, wait)
	m.stopServers()

	if wait {
		log.Infof("%s waiting for the wait group to finish", m)
		m.wg.Wait()
		log.Infof("%s wait group finished", m)
	}
}

func (m *mux) stopServers() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.state == stateShuttingDown {
		log.Infof("%v is already shutting down", m)
		return
	}

	prevState := m.state
	m.state = stateShuttingDown
	close(m.stopC)

	// init state has no running servers, no need to close them
	if prevState == stateInit {
		return
	}

	for _, s := range m.servers {
		s.shutdown()
	}
}

func (m *mux) UpsertHost(hostCfg engine.Host) error {
	log.Infof("%s UpsertHost %s", m, &hostCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.hosts[engine.HostKey{Name: hostCfg.Name}] = hostCfg

	for _, s := range m.servers {
		if s.isTLS() {
			s.reload()
		}
	}
	return nil
}

func (m *mux) DeleteHost(hostKey engine.HostKey) error {
	log.Infof("%s DeleteHost %v", m, &hostKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	host, exists := m.hosts[hostKey]
	if !exists {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", hostKey)}
	}

	// delete host from the hosts list
	delete(m.hosts, hostKey)

	// delete staple from the cache
	m.stapler.DeleteHost(hostKey)

	if host.Settings.KeyPair == nil {
		return nil
	}

	for _, s := range m.servers {
		s.reload()
	}
	return nil
}

func (m *mux) UpsertListener(listenerCfg engine.Listener) error {
	log.Infof("%v UpsertListener %v", m, &listenerCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.upsertListener(listenerCfg)
}

func (m *mux) DeleteListener(listenerKey engine.ListenerKey) error {
	log.Infof("%v DeleteListener %v", m, &listenerKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	s, exists := m.servers[listenerKey]
	if !exists {
		return &engine.NotFoundError{Message: fmt.Sprintf("%v not found", listenerKey)}
	}

	delete(m.servers, listenerKey)
	s.shutdown()
	return nil
}

func (m *mux) upsertListener(listenerCfg engine.Listener) error {
	listenerKey := engine.ListenerKey{Id: listenerCfg.Id}
	feSrv, exists := m.servers[listenerKey]
	if exists {
		return feSrv.updateListener(listenerCfg)
	}

	// Check if there's a listener with the same address
	for _, srv := range m.servers {
		if srv.listener.Address == listenerCfg.Address {
			return &engine.AlreadyExistsError{Message: fmt.Sprintf("%v conflicts with existing %v", listenerCfg, srv.listener)}
		}
	}

	var err error
	if feSrv, err = newSrv(m, listenerCfg); err != nil {
		return err
	}
	m.servers[listenerKey] = feSrv
	// If we are active, start the server immediatelly
	if m.state == stateActive {
		log.Infof("Mux is in active state, starting the HTTP server")
		if err := feSrv.start(); err != nil {
			return err
		}
	}
	return nil
}

func (m *mux) UpsertBackend(beCfg engine.Backend) error {
	log.Infof("%v UpsertBackend %v", m, &beCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	beKey := engine.BackendKey{Id: beCfg.Id}
	beEnt, ok := m.backends[beKey]
	if ok {
		mutated, err := beEnt.backend.update(beCfg, m.options)
		if err != nil {
			return errors.Wrapf(err, "failed to update backend %v", beKey.Id)
		}
		if mutated {
			for _, fe := range beEnt.frontends {
				fe.onBackendMutated()
			}
		}
		return nil
	}

	beEnt, err := newBackendEntry(beCfg, m.options, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create backend %v", beKey.Id)
	}
	m.backends[beKey] = beEnt
	return nil
}

func (m *mux) DeleteBackend(beKey engine.BackendKey) error {
	log.Infof("%v DeleteBackend %s", m, &beKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	beEnt, ok := m.backends[beKey]
	if !ok {
		return errors.Errorf("backend missing %v", beKey.Id)
	}

	// Delete backend from being referenced - it is no longer in etcd and
	// future frontend additions to etcd shouldn't see a magical backend just
	// because vulcan is holding a reference to it.
	delete(m.backends, beKey)

	if len(beEnt.frontends) != 0 {
		return errors.Errorf("%v is used by frontends: %v", beEnt.backend.id, beEnt.frontends)
	}

	beEnt.backend.close()
	return nil
}

func (m *mux) UpsertFrontend(feCfg engine.Frontend) error {
	log.Infof("%v UpsertFrontend %v", m, &feCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	beEnt, ok := m.backends[engine.BackendKey{Id: feCfg.BackendId}]
	if !ok {
		return errors.Errorf("missing backend %v referenced by frontend %v", feCfg.BackendId, feCfg.Id)
	}

	feKey := engine.FrontendKey{Id: feCfg.Id}
	fe, ok := m.frontends[feKey]
	if ok {
		if feCfg.BackendId != fe.backend.id {
			oldBeEnt, ok := m.backends[engine.BackendKey{Id: fe.backend.id}]
			if ok {
				delete(oldBeEnt.frontends, feKey)
			} else {
				log.Warnf("Missing backend %v referenced by frontend %v", fe.backend.id, feCfg.Id)
			}
			beEnt.frontends[feKey] = fe
		}

		oldRoute := fe.cfg.Route
		if oldRoute != feCfg.Route {
			log.Infof("updating route from %v to %v", oldRoute, feCfg.Route)
			if err := m.router.Remove(oldRoute); err != nil {
				log.Errorf("Failed to remove route %v for frontend %v", oldRoute, feCfg.Id)
			}
		}
		fe.update(feCfg, beEnt.backend)
		if oldRoute != feCfg.Route {
			if err := m.router.Handle(feCfg.Route, fe); err != nil {
				return errors.Wrapf(err, "cannot add route %v for frontend %v", feCfg.Route, feCfg.Id)
			}
		}
		return nil
	}
	fe = newFrontend(feCfg, beEnt.backend, m.options, nil, m.outgoingConnTracker)
	m.frontends[feKey] = fe
	beEnt.frontends[feKey] = fe
	if err := m.router.Handle(feCfg.Route, fe); err != nil {
		return errors.Wrapf(err, "cannot add route %v for frontend %v", feCfg.Route, feCfg.Id)
	}
	return nil
}

func (m *mux) DeleteFrontend(feKey engine.FrontendKey) error {
	log.Infof("%v DeleteFrontend %v", m, &feKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	fe, ok := m.frontends[feKey]
	if !ok {
		return errors.Errorf("missing frontend %v", feKey.Id)
	}

	m.router.Remove(fe.cfg.Route)
	delete(m.frontends, feKey)

	beEnt, ok := m.backends[engine.BackendKey{Id: fe.backend.id}]
	if !ok {
		return errors.Errorf("missing backend %v referenced by frontend %v", fe.backend.id, fe.cfg.Id)
	}
	delete(beEnt.frontends, feKey)
	return nil
}

func (m *mux) UpsertMiddleware(feKey engine.FrontendKey, mwCfg engine.Middleware) error {
	log.Infof("%v UpsertMiddleware %v, %v", m, &feKey, &mwCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	fe, ok := m.frontends[feKey]
	if !ok {
		return errors.Errorf("missing frontend %v referenced by middleware %v", feKey.Id, mwCfg.Id)
	}
	fe.upsertMiddleware(mwCfg)
	return nil
}

func (m *mux) DeleteMiddleware(mwKey engine.MiddlewareKey) error {
	log.Infof("%v DeleteMiddleware(%v %v)", m, &mwKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	fe, ok := m.frontends[mwKey.FrontendKey]
	if !ok {
		return errors.Errorf("missing frontend %v referenced by middleware %v", mwKey.FrontendKey.Id, mwKey.Id)
	}
	fe.deleteMiddleware(mwKey)
	return nil
}

func (m *mux) UpsertServer(beKey engine.BackendKey, beSrvCfg engine.Server) error {
	log.Infof("%v UpsertServer %v %v", m, &beKey, &beSrvCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, err := url.ParseRequestURI(beSrvCfg.URL); err != nil {
		return errors.Wrapf(err, "failed to parse %v", beSrvCfg)
	}

	beEnt, ok := m.backends[beKey]
	// If backend type is unknown then assume insecure HTTP.
	if !ok {
		beCfg := engine.Backend{Id: beKey.Id, Type: engine.HTTP, Settings: engine.HTTPBackendSettings{}}
		beEnt, _ = newBackendEntry(beCfg, m.options, nil)
		m.backends[beKey] = beEnt
	}
	if beEnt.backend.upsertServer(beSrvCfg) {
		for _, fe := range beEnt.frontends {
			fe.onBackendMutated()
		}
	}
	return nil
}

func (m *mux) DeleteServer(feSrvKey engine.ServerKey) error {
	log.Infof("%v DeleteServer %v", m, &feSrvKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	beEnt, ok := m.backends[feSrvKey.BackendKey]
	if !ok {
		return errors.Errorf("missing backend %v ", feSrvKey.BackendKey.Id)
	}
	if beEnt.backend.deleteServer(feSrvKey) {
		for _, fe := range beEnt.frontends {
			fe.onBackendMutated()
		}
	}
	return nil
}

func (m *mux) processStapleUpdate(e *stapler.StapleUpdated) error {
	log.Infof("%v processStapleUpdate event: %v", m, e)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, ok := m.hosts[e.HostKey]; !ok {
		log.Infof("%v %v from the staple update is not found, skipping", m, e.HostKey)
		return nil
	}

	for _, feSrv := range m.servers {
		if feSrv.isTLS() {
			// each server will ask stapler for the new OCSP response during reload
			feSrv.reload()
		}
	}
	return nil
}

type muxState int

const (
	stateInit         = iota // Server has been created, but does not accept connections yet
	stateActive              // Server is active and accepting connections
	stateShuttingDown        // Server is active, but is draining existing connections and does not accept new connections.
)

func (s muxState) String() string {
	switch s {
	case stateInit:
		return "init"
	case stateActive:
		return "active"
	case stateShuttingDown:
		return "shutting down"
	}
	return "undefined"
}

func setDefaults(o Options) Options {
	if o.MetricsClient == nil {
		o.MetricsClient = metrics.NewNop()
	}
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	if o.Router == nil {
		o.Router = route.NewMux()
	}
	if o.IncomingConnectionTracker == nil {
		o.IncomingConnectionTracker = newDefaultConnTracker()
	}
	return o
}

// NotFound is a generic http.Handler for request
type DefaultNotFound struct {
}

// ServeHTTP returns a simple 404 Not found response
func (*DefaultNotFound) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Infof("Not found: %v %v", r.Method, r.URL)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, `{"error":"not found"}`)
}
