package server

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route/exproute"

	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/connwatch"
	. "github.com/mailgun/vulcand/endpoint"
)

// MuxServer is capable of listening on multiple interfaces, graceful shutdowns and updating TLS certificates
type MuxServer struct {
	// Debugging id
	id int
	// Each listener address has a server associated with it
	servers map[backend.Address]*server

	// Options hold parameters that are used to initialize http servers
	options Options

	// Wait group for graceful shutdown
	wg *sync.WaitGroup

	// Read write mutex for serlialized operations
	mtx *sync.RWMutex

	// Host routers will be shared between mulitple listeners
	hostRouters map[string]*exproute.ExpRouter

	// Current server stats
	state int

	// Connection watcher
	connWatcher *connwatch.ConnectionWatcher
}

func (m *MuxServer) String() string {
	return fmt.Sprintf("MuxServer(%d, %s)", m.id, stateDescription(m.state))
}

func NewMuxServerWithOptions(id int, o Options) (*MuxServer, error) {
	return &MuxServer{
		id:          id,
		hostRouters: make(map[string]*exproute.ExpRouter),
		servers:     make(map[backend.Address]*server),
		options:     o,
		connWatcher: connwatch.NewConnectionWatcher(),
		wg:          &sync.WaitGroup{},
		mtx:         &sync.RWMutex{},
	}, nil
}

func (m *MuxServer) GetConnWatcher() *connwatch.ConnectionWatcher {
	return m.connWatcher
}

func (m *MuxServer) GetStats(hostname, locationId string, e *backend.Endpoint) *backend.EndpointStats {
	rr := m.getLocationLB(hostname, locationId)
	if rr == nil {
		return nil
	}
	endpoint := rr.FindEndpointById(e.GetUniqueId())
	if endpoint == nil {
		return nil
	}
	meterI := endpoint.GetMeter()
	if meterI == nil {
		return nil
	}
	meter := meterI.(*metrics.RollingMeter)

	return &backend.EndpointStats{
		Successes:     meter.SuccessCount(),
		Failures:      meter.FailureCount(),
		PeriodSeconds: int(meter.GetWindowSize() / time.Second),
		FailRate:      meter.GetRate(),
	}
}

func (m *MuxServer) HijackListenersFrom(o Server) error {
	log.Infof("%s HijackListenersFrom %s", m, o)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	other, ok := o.(*MuxServer)
	if !ok {
		return fmt.Errorf("can hijack listeners only from other MuxServer")
	}

	for addr, srv := range m.servers {
		osrv, exists := other.servers[addr]
		if !exists || !osrv.hasListeners() {
			log.Infof("Skipping hijack for address %s, has no active listeners", addr)
			continue
		}
		if err := srv.hijackListenerFrom(osrv); err != nil {
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

func (m *MuxServer) UpsertHost(host *backend.Host) error {
	log.Infof("%s UpsertHost(%s)", m, host)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if err := m.checkShuttingDown(); err != nil {
		return err
	}

	return m.upsertHost(host)
}

func (m *MuxServer) UpdateHostCert(hostname string, cert *backend.Certificate) error {
	log.Infof("%s UpdateHostCert(%s)", m, hostname)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, s := range m.servers {
		if s.hasHost(hostname) && s.isTLS() {
			if err := s.updateHostCert(hostname, cert); err != nil {
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

	return m.upsertLocation(host, loc)
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

	if err := m.upsertLocation(host, loc); err != nil {
		return err
	}
	return m.syncLocationEndpoints(loc)
}

func (m *MuxServer) UpdateLocationPath(host *backend.Host, loc *backend.Location, path string) error {
	log.Infof("%s UpdateLocationPath %s %s %s", m, host, loc, path)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	// If location already exists, delete it and re-create from scratch
	if httploc := m.getLocation(host.Name, loc.Id); httploc != nil {
		if err := m.deleteLocation(host, loc.Id); err != nil {
			return err
		}
	}
	return m.upsertLocation(host, loc)
}

func (m *MuxServer) UpdateLocationOptions(host *backend.Host, loc *backend.Location) error {
	log.Infof("%s UpdateLocationOptions %s %s %s", m, host, loc, loc.Options)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	if err := m.upsertLocation(host, loc); err != nil {
		return err
	}
	location := m.getLocation(host.Name, loc.Id)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
	}
	options, err := m.getLocationOptions(loc)
	if err != nil {
		return err
	}
	return location.SetOptions(*options)
}

func (m *MuxServer) DeleteLocation(host *backend.Host, locationId string) error {
	log.Infof("%s DeleteLocation %s %s", m, host, locationId)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.deleteLocation(host, locationId)
}

func (m *MuxServer) UpsertEndpoint(upstream *backend.Upstream, e *backend.Endpoint, affectedLocations []*backend.Location) error {
	log.Infof("%s UpsertEdpoint %s %s", m, upstream, e)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	return m.addEndpoint(upstream, e, affectedLocations)
}

func (m *MuxServer) DeleteEndpoint(upstream *backend.Upstream, endpointId string, affectedLocations []*backend.Location) error {
	log.Infof("%s DeleteEndpoint %s %s", m, upstream, endpointId)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, l := range affectedLocations {
		if err := m.syncLocationEndpoints(l); err != nil {
			log.Errorf("Failed to sync %s endpoints err: %s", l, err)
		}
	}
	return nil
}

func (m *MuxServer) getLocationOptions(loc *backend.Location) (*httploc.Options, error) {
	o, err := loc.GetOptions()
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

func (m *MuxServer) getRouter(hostname string) *exproute.ExpRouter {
	return m.hostRouters[hostname]
}

func (m *MuxServer) getLocation(hostname string, locationId string) *httploc.HttpLocation {
	router := m.getRouter(hostname)
	if router == nil {
		return nil
	}
	ilo := router.GetLocationById(locationId)
	if ilo == nil {
		return nil
	}
	return ilo.(*httploc.HttpLocation)
}

func (m *MuxServer) getLocationLB(hostname string, locationId string) *roundrobin.RoundRobin {
	loc := m.getLocation(hostname, locationId)
	if loc == nil {
		return nil
	}
	return loc.GetLoadBalancer().(*roundrobin.RoundRobin)
}

func (m *MuxServer) upsertLocation(host *backend.Host, loc *backend.Location) error {
	if err := m.upsertHost(host); err != nil {
		return err
	}

	// If location already exists, do nothing
	if loc := m.getLocation(host.Name, loc.Id); loc != nil {
		return nil
	}

	router := m.getRouter(host.Name)
	if router == nil {
		return fmt.Errorf("router not found for %s", host)
	}
	// Create a load balancer that handles all the endpoints within the given location
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return err
	}

	// Create a location itself
	options, err := m.getLocationOptions(loc)
	if err != nil {
		return err
	}
	location, err := httploc.NewLocationWithOptions(loc.Id, rr, *options)
	if err != nil {
		return err
	}

	// Always register a global connection watcher
	location.GetObserverChain().Upsert(ConnWatch, m.connWatcher)

	// Add the location to the router
	if err := router.AddLocation(convertPath(loc.Path), location); err != nil {
		return err
	}

	// Add middlewares
	for _, ml := range loc.Middlewares {
		if err := m.upsertLocationMiddleware(host, loc, ml); err != nil {
			log.Errorf("failed to add middleware: %s", err)
		}
	}
	// Once the location added, configure all endpoints
	return m.syncLocationEndpoints(loc)
}

func (m *MuxServer) upsertLocationMiddleware(host *backend.Host, loc *backend.Location, mi *backend.MiddlewareInstance) error {
	if err := m.upsertLocation(host, loc); err != nil {
		return err
	}
	location := m.getLocation(host.Name, loc.Id)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
	}
	instance, err := mi.Middleware.NewMiddleware()
	if err != nil {
		return err
	}
	location.GetMiddlewareChain().Upsert(fmt.Sprintf("%s.%s", mi.Type, mi.Id), mi.Priority, instance)
	return nil
}

func (m *MuxServer) deleteLocationMiddleware(host *backend.Host, loc *backend.Location, mType, mId string) error {
	location := m.getLocation(host.Name, loc.Id)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
	}
	return location.GetMiddlewareChain().Remove(fmt.Sprintf("%s.%s", mType, mId))
}

func (m *MuxServer) syncLocationEndpoints(location *backend.Location) error {

	rr := m.getLocationLB(location.Hostname, location.Id)
	if rr == nil {
		return fmt.Errorf("%s lb not found", location)
	}

	// First, collect and parse endpoints to add
	newEndpoints := map[string]endpoint.Endpoint{}
	for _, e := range location.Upstream.Endpoints {
		ep, err := EndpointFromUrl(e.GetUniqueId(), e.Url)
		if err != nil {
			return fmt.Errorf("Failed to parse endpoint url: %s", e)
		}
		newEndpoints[e.Url] = ep
	}

	// Memorize what endpoints exist in load balancer at the moment
	existingEndpoints := map[string]endpoint.Endpoint{}
	for _, e := range rr.GetEndpoints() {
		existingEndpoints[e.GetUrl().String()] = e
	}

	// First, add endpoints, that should be added and are not in lb
	for _, e := range newEndpoints {
		if _, exists := existingEndpoints[e.GetUrl().String()]; !exists {
			if err := rr.AddEndpoint(e); err != nil {
				log.Errorf("%s failed to add %s, err: %s", m, e, err)
			} else {
				log.Infof("%s add endpoint %s to %s", m, e, location)
			}
		}
	}

	// Second, remove endpoints that should not be there any more
	for _, e := range existingEndpoints {
		if _, exists := newEndpoints[e.GetUrl().String()]; !exists {
			if err := rr.RemoveEndpoint(e); err != nil {
				log.Errorf("Failed to remove %s, err: %s", e, err)
			} else {
				log.Infof("Removed %s from %s", e, location)
			}
		}
	}
	return nil
}

func (m *MuxServer) addEndpoint(upstream *backend.Upstream, e *backend.Endpoint, affectedLocations []*backend.Location) error {
	endpoint, err := EndpointFromUrl(e.GetUniqueId(), e.Url)
	if err != nil {
		return fmt.Errorf("Failed to parse endpoint url: %s", endpoint)
	}
	for _, l := range affectedLocations {
		if err := m.syncLocationEndpoints(l); err != nil {
			log.Errorf("Failed to sync %s endpoints err: %s", l, err)
		}
	}
	return nil
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
	router := m.getRouter(host.Name)
	if router == nil {
		return fmt.Errorf("Router for %s not found", host)
	}

	location := router.GetLocationById(locationId)
	if location == nil {
		return fmt.Errorf("location(id=%s) not found", locationId)
	}
	return router.RemoveLocationById(location.GetId())
}

func (m *MuxServer) checkShuttingDown() error {
	if m.state == stateShuttingDown {
		return fmt.Errorf("MuxServer is shutting down, ignore all operations")
	}
	return nil
}

const (
	stateInit         = iota // Server has been created, but does not accept connections yet
	stateActive              // Server is active and accepting connections
	stateShuttingDown        // Server is active, but is draining existing connections and does not accept new connections.
)

func stateDescription(state int) string {
	switch state {
	case stateInit:
		return "init"
	case stateActive:
		return "active"
	case stateShuttingDown:
		return "shutting down"
	}
	return "undefined"
}

const ConnWatch = "_vulcanConnWatch"

// convertPath changes strings to structured format /hello -> RegexpRoute("/hello") and leaves structured strings unchanged.
func convertPath(in string) string {
	if !strings.Contains(in, exproute.TrieRouteFn) && !strings.Contains(in, exproute.RegexpRouteFn) {
		return fmt.Sprintf(`%s(%#v)`, exproute.RegexpRouteFn, in)
	}
	return in
}
