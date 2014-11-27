package server

import (
	"fmt"
	"sync"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route/exproute"

	"github.com/mailgun/vulcand/backend"
)

// MuxServer is capable of listening on multiple interfaces, graceful shutdowns and updating TLS certificates
type MuxServer struct {
	// Debugging id
	id int
	// Each listener address has a server associated with it
	servers map[backend.Address]*server

	// Each upstream holds a transport and list of associated locations
	upstreams map[backend.UpstreamKey]*upstream

	locations map[backend.LocationKey]*location

	// Options hold parameters that are used to initialize http servers
	options Options

	// Wait group for graceful shutdown
	wg *sync.WaitGroup

	// Read write mutex for serlialized operations
	mtx *sync.RWMutex

	// Host routers will be shared between mulitple listeners
	hostRouters map[string]*exproute.ExpRouter

	// Current server stats
	state muxState

	// Connection watcher
	connTracker *connTracker

	// Perfomance monitor
	perfMon *perfMon
}

func (m *MuxServer) String() string {
	return fmt.Sprintf("MuxServer(%d, %v)", m.id, m.state)
}

func NewMuxServerWithOptions(id int, o Options) (*MuxServer, error) {
	o = parseOptions(o)
	return &MuxServer{
		id:          id,
		hostRouters: make(map[string]*exproute.ExpRouter),
		servers:     make(map[backend.Address]*server),
		options:     o,
		connTracker: newConnTracker(o.MetricsClient),
		wg:          &sync.WaitGroup{},
		mtx:         &sync.RWMutex{},
		perfMon:     newPerfMon(o.TimeProvider),
		upstreams:   make(map[backend.UpstreamKey]*upstream),
		locations:   make(map[backend.LocationKey]*location),
	}, nil
}

func (m *MuxServer) GetFiles() ([]*FileDescriptor, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	fds := []*FileDescriptor{}

	for _, srv := range m.servers {
		fd, err := srv.GetFile()
		if err != nil {
			return nil, err
		}
		if fd != nil {
			fds = append(fds, fd)
		}
	}
	return fds, nil
}

func (m *MuxServer) GetLocationStats(l *backend.Location) (*backend.RoundTripStats, error) {
	return m.perfMon.getLocationStats(l)
}

func (m *MuxServer) GetEndpointStats(e *backend.Endpoint) (*backend.RoundTripStats, error) {
	return m.perfMon.getEndpointStats(e)
}

func (m *MuxServer) GetUpstreamStats(u *backend.Upstream) (*backend.RoundTripStats, error) {
	return m.perfMon.getUpstreamStats(u)
}

func (m *MuxServer) GetTopLocations(hostname, upstreamId string) ([]*backend.Location, error) {
	return m.perfMon.getTopLocations(hostname, upstreamId)
}

func (m *MuxServer) GetTopEndpoints(upstreamId string) ([]*backend.Endpoint, error) {
	return m.perfMon.getTopEndpoints(upstreamId)
}

func (m *MuxServer) TakeFiles(files []*FileDescriptor) error {
	log.Infof("%s TakeFiles %s", m, files)

	fMap := make(map[backend.Address]*FileDescriptor, len(files))
	for _, f := range files {
		fMap[f.Address] = f
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()

	for addr, srv := range m.servers {
		file, exists := fMap[addr]
		if !exists {
			log.Infof("%s skipping take of files from address %s, has no passed files", m, addr)
			continue
		}
		if err := srv.takeFile(file); err != nil {
			return err
		}
	}

	return nil
}

func (m *MuxServer) Start() error {
	log.Infof("%s start", m)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.state != stateInit {
		return fmt.Errorf("%s can start only from init state, got %d", m, m.state)
	}

	m.state = stateActive
	for _, s := range m.servers {
		if err := s.start(); err != nil {
			return err
		}
	}

	log.Infof("%s started", m)
	return nil
}

func (m *MuxServer) Stop(wait bool) {
	log.Infof("%s Stop(%t)", m, wait)

	m.stopServers()

	if wait {
		log.Infof("%s waiting for the wait group to finish", m)
		m.wg.Wait()
		log.Infof("%s wait group finished", m)
	}
}

func (m *MuxServer) stopServers() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.state == stateInit {
		m.state = stateShuttingDown
		return
	}

	if m.state == stateShuttingDown {
		return
	}

	m.state = stateShuttingDown
	for _, s := range m.servers {
		s.shutdown()
	}
}

func (m *MuxServer) UpsertUpstream(u *backend.Upstream) error {
	log.Infof("%v UpsertUpstream(%v)", m, u)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	_, err := m.upsertUpstream(u)
	return err
}

func (m *MuxServer) upsertUpstream(u *backend.Upstream) (*upstream, error) {
	up, ok := m.upstreams[u.GetUniqueId()]
	if ok {
		return up, up.update(u)
	}
	up, err := newUpstream(m, u)
	if err != nil {
		return nil, err
	}
	m.upstreams[u.GetUniqueId()] = up
	return up, nil
}

func (m *MuxServer) DeleteUpstream(upstreamId string) error {
	log.Infof("%v DeleteUpstream(%s)", m, upstreamId)

	up, ok := m.upstreams[backend.UpstreamKey{Id: upstreamId}]
	if !ok {
		return fmt.Errorf("Upstream(%v) not found", upstreamId)
	}

	if len(up.locs) != 0 {
		return fmt.Errorf("Upstream(%v) is used by locations: %v", upstreamId, up.locs)
	}

	up.Close()
	m.perfMon.deleteUpstream(backend.UpstreamKey{Id: upstreamId})
	return nil
}

func (m *MuxServer) UpsertHost(host *backend.Host) error {
	log.Infof("%s UpsertHost(%s)", m, host)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.upsertHost(host)
}

func (m *MuxServer) UpdateHostKeyPair(hostname string, keyPair *backend.KeyPair) error {
	log.Infof("%s UpdateHostKeyPair(%s)", m, hostname)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, s := range m.servers {
		if s.hasHost(hostname) && s.isTLS() {
			if err := s.updateHostKeyPair(hostname, keyPair); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MuxServer) AddHostListener(h *backend.Host, l *backend.Listener) error {
	log.Infof("%s AddHostLsitener %s %s", m, h, l)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if err := m.upsertHost(h); err != nil {
		return err
	}
	if m.hasHostListener(h.Name, l.Id) {
		return nil
	}
	return m.addHostListener(h, m.hostRouters[h.Name], l)
}

func (m *MuxServer) DeleteHostListener(host *backend.Host, listenerId string) error {
	log.Infof("%s DeleteHostListener %s %s", m, host.Name, listenerId)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	var err error
	for k, s := range m.servers {
		if s.hasListener(host.Name, listenerId) {
			closed, e := s.deleteHost(host.Name)
			if closed {
				log.Infof("Closed server listening on %s", k)
				delete(m.servers, k)
			}
			err = e
		}
	}
	return err
}

func (m *MuxServer) DeleteHost(hostname string) error {
	log.Infof("%s DeleteHost Host(%s)", m, hostname)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, s := range m.servers {
		closed, err := s.deleteHost(hostname)
		if err != nil {
			return err
		}
		if closed {
			log.Infof("%s was closed", s)
			delete(m.servers, s.listener.Address)
		}
	}

	delete(m.hostRouters, hostname)
	return nil
}

func (m *MuxServer) UpsertLocation(host *backend.Host, loc *backend.Location) error {
	log.Infof("%s UpsertLocation %s %s", m, host, loc)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	_, err := m.upsertLocation(host, loc)
	return err
}

func (m *MuxServer) UpsertLocationMiddleware(host *backend.Host, loc *backend.Location, mi *backend.MiddlewareInstance) error {
	log.Infof("%s UpsertLocationMiddleware %s %s %s", m, host, loc, mi)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.upsertLocationMiddleware(host, loc, mi)
}

func (m *MuxServer) DeleteLocationMiddleware(host *backend.Host, loc *backend.Location, mType, mId string) error {
	log.Infof("%s DeleteLocationMiddleware %s %s %s %s", m, host, loc, mType, mId)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.deleteLocationMiddleware(host, loc, mType, mId)
}

func (m *MuxServer) UpdateLocationUpstream(host *backend.Host, loc *backend.Location) error {
	log.Infof("%s UpdateLocationUpstream %s %s", m, host, loc)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	_, err := m.upsertLocation(host, loc)
	return err
}

func (m *MuxServer) UpdateLocationPath(host *backend.Host, loc *backend.Location, path string) error {
	log.Infof("%s UpdateLocationPath %s %s %s", m, host, loc, path)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// If location already exists, delete it and re-create from scratch
	if _, ok := m.locations[loc.GetUniqueId()]; ok {
		if err := m.deleteLocation(host, loc.Id); err != nil {
			return err
		}
	}
	_, err := m.upsertLocation(host, loc)
	return err
}

func (m *MuxServer) UpdateLocationOptions(host *backend.Host, loc *backend.Location) error {
	log.Infof("%s UpdateLocationOptions %s %s %s", m, host, loc, loc.Options)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	l, ok := m.locations[loc.GetUniqueId()]
	if !ok {
		return fmt.Errorf("%v not found", loc)
	}

	return l.updateOptions(loc)
}

func (m *MuxServer) DeleteLocation(host *backend.Host, locationId string) error {
	log.Infof("%s DeleteLocation %s %s", m, host, locationId)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.deleteLocation(host, locationId)
}

func (m *MuxServer) UpsertEndpoint(upstream *backend.Upstream, e *backend.Endpoint) error {
	log.Infof("%s UpsertEndpoint %s %s", m, upstream, e)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if _, err := netutils.ParseUrl(e.Url); err != nil {
		return fmt.Errorf("failed to parse %v, error: %v", e, err)
	}

	up, ok := m.upstreams[upstream.GetUniqueId()]
	if !ok {
		return fmt.Errorf("%v not found", upstream)
	}

	return up.updateEndpoints(upstream.Endpoints)
}

func (m *MuxServer) DeleteEndpoint(upstream *backend.Upstream, endpointId string) error {
	log.Infof("%s DeleteEndpoint %s %s", m, upstream, endpointId)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	up, ok := m.upstreams[upstream.GetUniqueId()]
	if !ok {
		return fmt.Errorf("%v not found", upstream)
	}

	return up.updateEndpoints(upstream.Endpoints)
}

func (m *MuxServer) getRouter(hostname string) *exproute.ExpRouter {
	return m.hostRouters[hostname]
}

func (m *MuxServer) getLocation(hostname string, locationId string) *httploc.HttpLocation {
	l, ok := m.locations[backend.LocationKey{Hostname: hostname, Id: locationId}]
	if !ok {
		return nil
	}
	return l.hloc
}

func (m *MuxServer) upsertLocation(host *backend.Host, loc *backend.Location) (*location, error) {
	if err := m.upsertHost(host); err != nil {
		return nil, err
	}

	up, err := m.upsertUpstream(loc.Upstream)
	if err != nil {
		return nil, err
	}

	l, ok := m.locations[loc.GetUniqueId()]
	// If location already exists, update its upstream
	if ok {
		return l, l.updateUpstream(up)
	}

	// create a new location
	l, err = newLocation(m, loc, up)
	if err != nil {
		return nil, err
	}
	// register it with the locations registry
	m.locations[loc.GetUniqueId()] = l
	return l, nil
}

func (m *MuxServer) upsertLocationMiddleware(host *backend.Host, loc *backend.Location, mi *backend.MiddlewareInstance) error {
	l, err := m.upsertLocation(host, loc)
	if err != nil {
		return err
	}
	return l.upsertMiddleware(mi)
}

func (m *MuxServer) deleteLocationMiddleware(host *backend.Host, loc *backend.Location, mType, mId string) error {
	l, ok := m.locations[loc.GetUniqueId()]
	if !ok {
		return fmt.Errorf("%s not found", loc)
	}
	return l.deleteMiddleware(mType, mId)
}

func (m *MuxServer) addHostListener(host *backend.Host, router route.Router, l *backend.Listener) error {
	s, exists := m.servers[l.Address]
	if !exists {
		var err error
		if s, err = newServer(m, host, router, l); err != nil {
			return err
		}
		m.servers[l.Address] = s
		// If we are active, start the server immediatelly
		if m.state == stateActive {
			log.Infof("Mux is in active state, starting the HTTP server")
			if err := s.start(); err != nil {
				return err
			}
		}
		return nil
	}

	// We can not listen for different protocols on the same socket
	if s.listener.Protocol != l.Protocol {
		return fmt.Errorf("conflicting protocol %s and %s", s.listener.Protocol, l.Protocol)
	}

	return s.addHost(host, router, l)
}

func (m *MuxServer) upsertHost(host *backend.Host) error {
	if _, exists := m.hostRouters[host.Name]; exists {
		return nil
	}

	router := exproute.NewExpRouter()
	m.hostRouters[host.Name] = router

	if m.options.DefaultListener != nil {
		host.Listeners = append(host.Listeners, m.options.DefaultListener)
	}

	for _, l := range host.Listeners {
		if err := m.addHostListener(host, router, l); err != nil {
			return err
		}
	}

	return nil
}

func (m *MuxServer) hasHostListener(hostname, listenerId string) bool {
	for _, s := range m.servers {
		if s.hasListener(hostname, listenerId) {
			return true
		}
	}
	return false
}

func (m *MuxServer) deleteLocation(host *backend.Host, locationId string) error {
	key := backend.LocationKey{Hostname: host.Name, Id: locationId}
	l, ok := m.locations[key]
	if !ok {
		return fmt.Errorf("%v not found")
	}
	if err := l.remove(); err != nil {
		return err
	}
	delete(m.locations, key)
	return nil
}

func (m *MuxServer) getTransportOptions(up *backend.Upstream) (*httploc.TransportOptions, error) {
	o, err := up.GetTransportOptions()
	if err != nil {
		return nil, err
	}
	// Apply global defaults if options are not set
	if o.Timeouts.Dial == 0 {
		o.Timeouts.Dial = m.options.DialTimeout
	}
	if o.Timeouts.Read == 0 {
		o.Timeouts.Read = m.options.ReadTimeout
	}
	return o, nil
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

const (
	Metrics = "_metrics"
	PerfMon = "_perfMon"
)

func parseOptions(o Options) Options {
	if o.MetricsClient == nil {
		o.MetricsClient = metrics.NewNop()
	}
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	return o
}
