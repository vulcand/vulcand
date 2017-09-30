package mux

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mailgun/metrics"
	"github.com/mailgun/timetools"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/route"
	"github.com/vulcand/vulcand/conntracker"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/plugin"
	"github.com/vulcand/vulcand/proxy"
	"github.com/vulcand/vulcand/proxy/backend"
	"github.com/vulcand/vulcand/proxy/connctr"
	"github.com/vulcand/vulcand/proxy/frontend"
	"github.com/vulcand/vulcand/proxy/rtmcollect"
	"github.com/vulcand/vulcand/proxy/server"
	"github.com/vulcand/vulcand/router"
	"github.com/vulcand/vulcand/stapler"
)

// mux is capable of listening on multiple interfaces, graceful shutdowns and updating TLS certificates
type mux struct {
	// Debugging id
	id int

	// Each listener address has a server associated with it
	servers map[engine.ListenerKey]*server.T

	backends map[engine.BackendKey]backendEntry

	frontends map[engine.FrontendKey]*frontend.T

	hostCfgs map[engine.HostKey]engine.Host

	// Options hold parameters that are used to initialize http servers
	options proxy.Options

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

	frontendListeners plugin.FrontendListeners

	// stopC used for global broadcast to all proxy systems that it's closed
	stopC chan struct{}

	// OCSP staple cache and responder
	stapler stapler.Stapler

	// Unsubscribe from staple updates
	stapleUpdatesC chan *stapler.StapleUpdated
}

type backendEntry struct {
	backend   *backend.T
	frontends map[engine.FrontendKey]*frontend.T
}

func newBackendEntry(beCfg engine.Backend, opts proxy.Options, beSrvs []backend.Srv) (backendEntry, error) {
	be, err := backend.New(beCfg, opts, beSrvs)
	if err != nil {
		return backendEntry{}, errors.Wrap(err, "failed to create backend")
	}
	return backendEntry{
		backend:   be,
		frontends: make(map[engine.FrontendKey]*frontend.T),
	}, nil
}

func (m *mux) String() string {
	return fmt.Sprintf("mux_%d", m.id)
}

func New(id int, st stapler.Stapler, o proxy.Options) (*mux, error) {
	o = setDefaults(o)
	m := &mux{
		id:      id,
		options: o,

		router:              o.Router,
		incomingConnTracker: o.IncomingConnectionTracker,
		frontendListeners:   o.FrontendListeners,

		servers:   make(map[engine.ListenerKey]*server.T),
		backends:  make(map[engine.BackendKey]backendEntry),
		frontends: make(map[engine.FrontendKey]*frontend.T),
		hostCfgs:  make(map[engine.HostKey]engine.Host),

		stapleUpdatesC: make(chan *stapler.StapleUpdated),
		stopC:          make(chan struct{}),
		stapler:        st,
	}

	m.router.SetNotFound(proxy.DefaultNotFound)
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

	for _, hostCfg := range ss.Hosts {
		m.hostCfgs[hostCfg.Key()] = hostCfg
	}

	for _, bes := range ss.BackendSpecs {
		beKey := engine.BackendKey{Id: bes.Backend.Id}
		beSrvs := make([]backend.Srv, len(bes.Servers))
		for i, beSrvCfg := range bes.Servers {
			beSrv, err := backend.NewServer(beSrvCfg)
			if err != nil {
				return errors.Wrapf(err, "bad server config %v", beSrvCfg.Id)
			}
			beSrvs[i] = beSrv
		}
		beEnt, err := newBackendEntry(bes.Backend, m.options, beSrvs)
		if err != nil {
			return errors.Wrapf(err, "failed to create backend entry %v", bes.Backend.Id)
		}
		m.backends[beKey] = beEnt
	}

	for _, lsnCfg := range ss.Listeners {
		for _, srv := range m.servers {
			if srv.Address() == lsnCfg.Address {
				// This only exists to simplify test fixture configuration.
				if srv.Key() == lsnCfg.Key() {
					continue
				}
				return errors.Errorf("%v conflicts with existing %v", lsnCfg.Id, srv.Key())
			}
		}
		srv, err := server.New(lsnCfg, m.router, m.stapler, m.incomingConnTracker, &m.wg)
		if err != nil {
			return errors.Wrapf(err, "failed to create server %v", lsnCfg.Id)
		}
		m.servers[lsnCfg.Key()] = srv
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
		fe := frontend.New(fes.Frontend, beEnt.backend, m.options, mwCfgs, m.frontendListeners)
		if err := m.router.Handle(fes.Frontend.Route, fe); err != nil {
			return errors.Wrapf(err, "cannot add route %v for frontend %v",
				fes.Frontend.Route, fes.Frontend.Id)
		}
		m.frontends[feKey] = fe
		beEnt.frontends[feKey] = fe
	}
	return nil
}

func (m *mux) GetFiles() ([]*proxy.FileDescriptor, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	fds := []*proxy.FileDescriptor{}

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

func (m *mux) TakeFiles(files []*proxy.FileDescriptor) error {
	log.Infof("%s TakeFiles %s", m, files)

	fMap := make(map[engine.Address]*proxy.FileDescriptor, len(files))
	for _, f := range files {
		fMap[f.Address] = f
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, srv := range m.servers {

		file, exists := fMap[srv.Address()]
		if !exists {
			log.Infof("%s skipping take of files from address %s, has no passed files", m, srv.Address())
			continue
		}
		if err := srv.TakeFile(file, m.hostCfgs); err != nil {
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
	for _, srv := range m.servers {
		if err := srv.Start(m.hostCfgs); err != nil {
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
		s.Shutdown()
	}
}

func (m *mux) UpsertHost(hostCfg engine.Host) error {
	log.Infof("%s UpsertHost %s", m, &hostCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.hostCfgs[hostCfg.Key()] = hostCfg
	for _, srv := range m.servers {
		srv.OnHostsUpdated(m.hostCfgs)
	}
	return nil
}

func (m *mux) DeleteHost(hostKey engine.HostKey) error {
	log.Infof("%s DeleteHost %v", m, &hostKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	host, ok := m.hostCfgs[hostKey]
	if !ok {
		return errors.Errorf("host %v not found", hostKey)
	}
	delete(m.hostCfgs, hostKey)

	// Delete staple from the cache
	m.stapler.DeleteHost(hostKey)

	// If the host has no TLS config then there is no need for server reload.
	if host.Settings.KeyPair == nil {
		return nil
	}
	for _, srv := range m.servers {
		srv.OnHostsUpdated(m.hostCfgs)
	}
	return nil
}

func (m *mux) UpsertListener(listenerCfg engine.Listener) error {
	log.Infof("%v UpsertListener %v", m, &listenerCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.upsertListener(listenerCfg)
}

func (m *mux) DeleteListener(lsnKey engine.ListenerKey) error {
	log.Infof("%v DeleteListener %v", m, &lsnKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	srv, ok := m.servers[lsnKey]
	if !ok {
		return errors.Errorf("%v not found", lsnKey)
	}

	delete(m.servers, lsnKey)
	srv.Shutdown()
	return nil
}

func (m *mux) upsertListener(lsnCfg engine.Listener) error {
	srv, ok := m.servers[lsnCfg.Key()]
	if ok {
		if err := srv.Update(lsnCfg, m.hostCfgs); err != nil {
			return errors.Wrapf(err, "failed to update server %v", lsnCfg.Key())
		}
		return nil
	}

	// Check if there's a listener with the same address
	for _, srv := range m.servers {
		if srv.Address() == lsnCfg.Address {
			return errors.Errorf("listener %v conflicts with existing server %v", lsnCfg.Key(), srv.Key())
		}
	}
	// Create a new server for the listener.
	var err error
	if srv, err = server.New(lsnCfg, m.router, m.stapler, m.incomingConnTracker, &m.wg); err != nil {
		return errors.Wrapf(err, "cannot create server %v", lsnCfg.Key())
	}
	m.servers[lsnCfg.Key()] = srv
	// Start the created server if the multipler is active.
	if m.state == stateActive {
		log.Infof("Mux is in active state, starting the HTTP server")
		if err := srv.Start(m.hostCfgs); err != nil {
			return errors.Wrapf(err, "failed to start server %v", lsnCfg.Key())
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
		mutated, err := beEnt.backend.Update(beCfg, m.options)
		if err != nil {
			return errors.Wrapf(err, "failed to update backend %v", beKey.Id)
		}
		if mutated {
			for _, fe := range beEnt.frontends {
				fe.OnBackendMutated()
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
		return errors.Errorf("%v is used by frontends: %v", beEnt.backend.Key(), beEnt.frontends)
	}

	beEnt.backend.Close()
	return nil
}

func (m *mux) UpsertFrontend(feCfg engine.Frontend) error {
	log.Infof("%v UpsertFrontend %v", m, &feCfg)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	beEnt, ok := m.backends[feCfg.BackendKey()]
	if !ok {
		return errors.Errorf("missing backend %v referenced by frontend %v", feCfg.BackendId, feCfg.Id)
	}

	feKey := engine.FrontendKey{Id: feCfg.Id}
	fe, ok := m.frontends[feKey]
	if ok {
		if feCfg.BackendKey() != fe.BackendKey() {
			oldBeEnt, ok := m.backends[fe.BackendKey()]
			if ok {
				delete(oldBeEnt.frontends, feKey)
			} else {
				log.Warnf("Missing backend %v referenced by frontend %v", fe.BackendKey(), feCfg.Key())
			}
			beEnt.frontends[feKey] = fe
		}

		oldRoute := fe.Route()
		if oldRoute != feCfg.Route {
			log.Infof("updating route from %v to %v", oldRoute, feCfg.Route)
			if err := m.router.Remove(oldRoute); err != nil {
				log.Errorf("Failed to remove route %v for frontend %v", oldRoute, feCfg.Id)
			}
		}
		if err := fe.Update(feCfg, beEnt.backend); err != nil {
			return errors.Wrapf(err, "failed to update fronend %v", feCfg.Key())
		}
		if oldRoute != feCfg.Route {
			if err := m.router.Handle(feCfg.Route, fe); err != nil {
				return errors.Wrapf(err, "cannot add route %v for frontend %v", feCfg.Route, feCfg.Id)
			}
		}
		return nil
	}
	fe = frontend.New(feCfg, beEnt.backend, m.options, nil, m.frontendListeners)
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

	m.router.Remove(fe.Route())
	delete(m.frontends, feKey)

	beEnt, ok := m.backends[fe.BackendKey()]
	if !ok {
		return errors.Errorf("missing backend %v referenced by frontend %v", fe.BackendKey(), fe.Key())
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
	fe.UpsertMiddleware(mwCfg)
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
	fe.DeleteMiddleware(mwKey)
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
	mutated, err := beEnt.backend.UpsertServer(beSrvCfg)
	if err != nil {
		return errors.Wrapf(err, "failed to upsert server %v to backend %v", beSrvCfg, beEnt.backend.Key())
	}
	if mutated {
		for _, fe := range beEnt.frontends {
			fe.OnBackendMutated()
		}
	}
	return nil
}

func (m *mux) DeleteServer(beSrvKey engine.ServerKey) error {
	log.Infof("%v DeleteServer %v", m, &beSrvKey)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	beEnt, ok := m.backends[beSrvKey.BackendKey]
	if !ok {
		return errors.Errorf("missing backend %v ", beSrvKey.BackendKey.Id)
	}
	if beEnt.backend.DeleteServer(beSrvKey) {
		for _, fe := range beEnt.frontends {
			fe.OnBackendMutated()
		}
	}
	return nil
}

func (m *mux) processStapleUpdate(e *stapler.StapleUpdated) {
	log.Infof("%v processStapleUpdate event: %v", m, e)
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, ok := m.hostCfgs[e.HostKey]; !ok {
		log.Infof("%v %v from the staple update is not found, skipping", m, e.HostKey)
		return
	}

	for _, srv := range m.servers {
		srv.OnHostsUpdated(m.hostCfgs)
	}
}

func (m *mux) emitMetrics() error {
	c := m.options.MetricsClient

	// Emit connection stats
	counts := m.incomingConnTracker.Counts()
	for state, values := range counts {
		for addr, count := range values {
			c.Gauge(c.Metric("conns", addr, state.String()), count, 1)
		}
	}

	// Emit frontend metrics stats
	frontends, err := m.TopFrontends(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get top frontends")
	}
	for _, fe := range frontends {
		fem := c.Metric("frontend", strings.Replace(fe.Id, ".", "_", -1))
		s := fe.Stats
		for _, scode := range s.Counters.StatusCodes {
			// response codes counters
			c.Gauge(fem.Metric("code", strconv.Itoa(scode.Code)), scode.Count, 1)
		}
		// network errors
		c.Gauge(fem.Metric("neterr"), s.Counters.NetErrors, 1)
		// requests
		c.Gauge(fem.Metric("reqs"), s.Counters.Total, 1)

		// round trip times in microsecond resolution
		for _, b := range s.LatencyBrackets {
			c.Gauge(fem.Metric("rtt", strconv.Itoa(int(b.Quantile*10.0))), int64(b.Value/time.Microsecond), 1)
		}
	}
	return nil
}

func (m *mux) FrontendStats(feKey engine.FrontendKey) (*engine.RoundTripStats, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	fe, ok := m.frontends[feKey]
	if !ok {
		return nil, errors.Errorf("%v not found", feKey)
	}
	feCfg, ok, err := fe.CfgWithStats()
	if err != nil {
		return nil, errors.Wrapf(err, "frontend %v RT stats not available", feKey)
	}
	if !ok {
		return nil, errors.Errorf("frontend %v RT not collected", feKey)
	}
	return feCfg.Stats, nil
}

func (m *mux) BackendStats(beKey engine.BackendKey) (*engine.RoundTripStats, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	beEnt, ok := m.backends[beKey]
	if !ok {
		return nil, errors.Errorf("backend %v not found", beKey)
	}

	aggregate := rtmcollect.NewRTMetrics()
	for _, fe := range beEnt.frontends {
		fe.AppendRTMTo(aggregate)
	}
	return engine.NewRoundTripStats(aggregate)
}

func (m *mux) ServerStats(beSrvKey engine.ServerKey) (*engine.RoundTripStats, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	beEnt, ok := m.backends[beSrvKey.BackendKey]
	if !ok {
		return nil, errors.Errorf("backend %v not found", beSrvKey.BackendKey)
	}
	beSrv, ok := beEnt.backend.Server(beSrvKey)
	if !ok {
		return nil, errors.Errorf("server %v not found", beSrvKey)
	}

	aggregates := rtmcollect.NewRTMetrics()
	for _, fe := range beEnt.frontends {
		fe.AppendBeSrvRTMTo(aggregates, beSrv.URLKey())
	}
	return engine.NewRoundTripStats(aggregates)
}

// TopFrontends returns locations sorted by criteria (faulty, slow, most used)
// if hostname or backendId is present, will filter out locations for that host or backendId
func (m *mux) TopFrontends(beKey *engine.BackendKey) ([]engine.Frontend, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	feCfgs := []engine.Frontend{}
	for _, fe := range m.filteredFrontends(beKey) {
		feCfg, ok, err := fe.CfgWithStats()
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get stats from %v", fe.Key())
		}
		if !ok {
			continue
		}
		feCfgs = append(feCfgs, feCfg)
	}
	sort.Stable(&frontendSorter{frontends: feCfgs})
	return feCfgs, nil
}

// TopServers returns endpoints sorted by criteria (faulty, slow, most used)
// if backendId is not empty, will filter out endpoints for that backendId
func (m *mux) TopServers(beKey *engine.BackendKey) ([]engine.Server, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	aggregates := make(map[backend.SrvURLKey]rtmcollect.BeSrvEntry)
	for _, fe := range m.filteredFrontends(beKey) {
		fe.AppendAllBeSrvRTMsTo(aggregates)
	}
	beSrvCfgs := make([]engine.Server, 0, len(aggregates))
	for _, beSrvEnt := range aggregates {
		beSrvCfgs = append(beSrvCfgs, beSrvEnt.CfgWithStats())
	}
	sort.Stable(&serverSorter{es: beSrvCfgs})
	return beSrvCfgs, nil
}

func (m *mux) filteredFrontends(beKey *engine.BackendKey) map[engine.FrontendKey]*frontend.T {
	if beKey != nil {
		if beEnt, ok := m.backends[*beKey]; ok {
			return beEnt.frontends
		}
	}
	return m.frontends
}

type frontendSorter struct {
	frontends []engine.Frontend
}

func (s *frontendSorter) Len() int {
	return len(s.frontends)
}

func (s *frontendSorter) Swap(i, j int) {
	s.frontends[i], s.frontends[j] = s.frontends[j], s.frontends[i]
}

func (s *frontendSorter) Less(i, j int) bool {
	return cmpStats(s.frontends[i].Stats, s.frontends[j].Stats)
}

type serverSorter struct {
	es []engine.Server
}

func (s *serverSorter) Len() int {
	return len(s.es)
}

func (s *serverSorter) Swap(i, j int) {
	s.es[i], s.es[j] = s.es[j], s.es[i]
}

func (s *serverSorter) Less(i, j int) bool {
	return cmpStats(s.es[i].Stats, s.es[j].Stats)
}

func cmpStats(s1, s2 *engine.RoundTripStats) bool {
	// Items that have network errors go first
	if s1.NetErrorRatio() != 0 || s2.NetErrorRatio() != 0 {
		return s1.NetErrorRatio() > s2.NetErrorRatio()
	}

	// Items that have application level errors go next
	if s1.AppErrorRatio() != 0 || s2.AppErrorRatio() != 0 {
		return s1.AppErrorRatio() > s2.AppErrorRatio()
	}

	// More highly loaded items go next
	return s1.Counters.Total > s2.Counters.Total
}

type muxState int

const (
	stateInit         = iota // Server has been created, but does not accept connections yet
	stateActive              // Server is active and accepting connections
	stateShuttingDown        // Server is active, but is draining existing connections and does not accept New connections.
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

func setDefaults(o proxy.Options) proxy.Options {
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
		o.IncomingConnectionTracker = connctr.New()
	}
	return o
}
