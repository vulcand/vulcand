package service

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/gorilla/mux"
	log "github.com/mailgun/gotools-log"
	runtime "github.com/mailgun/gotools-runtime"
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/callback"
	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/limit/connlimit"
	"github.com/mailgun/vulcan/limit/tokenbucket"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/route/hostroute"
	"github.com/mailgun/vulcan/route/pathroute"
	"github.com/mailgun/vulcand/api"
	. "github.com/mailgun/vulcand/backend"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"
)

type Service struct {
	client    *etcd.Client
	proxy     *vulcan.Proxy
	backend   Backend
	options   Options
	router    *hostroute.HostRouter
	apiRouter *mux.Router
	changes   chan *Change
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
		changes: make(chan *Change),
	}
}

func (s *Service) Start() error {
	// Init logging
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	backend, err := NewEtcdBackend(s.options.EtcdNodes, s.options.EtcdKey, s.options.EtcdConsistency, s.changes, s)
	if err != nil {
		return err
	}
	s.backend = backend

	if s.options.PidPath != "" {
		if err := runtime.WritePid(s.options.PidPath); err != nil {
			return fmt.Errorf("Failed to write PID file: %v\n", err)
		}
	}

	if err := s.createProxy(); err != nil {
		return err
	}
	if err := s.configureProxy(); err != nil {
		return err
	}
	if err := s.configureApi(); err != nil {
		return err
	}

	go s.startProxy()
	go s.startApi()
	go s.watchChanges()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	log.Infof("Got signal %s, exiting now", <-c)
	return nil
}

func (s *Service) GetStats(hostname, locationId, endpointId string) (*EndpointStats, error) {
	rr, err := s.getHttpLocationLb(hostname, locationId)
	if err != nil {
		return nil, err
	}
	endpoint := rr.FindEndpointById(endpointId)
	if endpoint == nil {
		return nil, fmt.Errorf("Endpoint: %s not found", endpointId)
	}
	weightedEndpoint, ok := endpoint.(*roundrobin.WeightedEndpoint)
	if !ok {
		return nil, fmt.Errorf("Unuspported endpoint type: %T", endpoint)
	}
	if weightedEndpoint == nil {
		return nil, fmt.Errorf("Weighted Endpoint: %s not found", endpointId)
	}
	metrics := weightedEndpoint.GetMetrics()
	if metrics == nil {
		return nil, fmt.Errorf("Metrics not found for endpoint %s", endpoint)
	}

	return &EndpointStats{
		Successes:     metrics.SuccessCount(),
		Failures:      metrics.FailureCount(),
		PeriodSeconds: int(metrics.Resolution() / time.Second),
		FailRate:      metrics.GetRate(),
	}, nil
}

func (s *Service) createProxy() error {
	s.router = hostroute.NewHostRouter()
	proxy, err := vulcan.NewProxy(s.router)
	if err != nil {
		return err
	}
	s.proxy = proxy
	return nil
}

func (s *Service) configureApi() error {
	s.apiRouter = mux.NewRouter()
	api.InitProxyController(s.backend, s.apiRouter)
	return nil
}

func (s *Service) configureProxy() error {
	hosts, err := s.backend.GetHosts()
	if err != nil {
		return err
	}
	for _, host := range hosts {
		log.Infof("Configuring %s", host)

		if err := s.addHost(host); err != nil {
			log.Errorf("Failed adding %s, err: %s", host, err)
			continue
		}
		if err := s.configureHost(host); err != nil {
			log.Errorf("Failed configuring %s", host)
			continue
		}
	}
	return nil
}

func (s *Service) configureHost(host *Host) error {
	for _, loc := range host.Locations {
		if err := s.addLocation(host, loc); err != nil {
			log.Errorf("Failed adding %s to %s, err: %s", loc, host, err)
		} else {
			log.Infof("Added %s to %s", loc, host)
		}
	}
	return nil
}

func (s *Service) configureLocation(host *Host, location *Location) error {
	rr, err := s.getHttpLocationLb(host.Name, location.Name)
	if err != nil {
		return err
	}

	// First, collect and parse endpoints to add
	endpointsToAdd := map[string]endpoint.Endpoint{}
	for _, e := range location.Upstream.Endpoints {
		ep, err := EndpointFromUrl(e.Name, e.Url)
		if err != nil {
			return fmt.Errorf("Failed to parse endpoint url: %s", e)
		}
		endpointsToAdd[ep.GetId()] = ep
	}

	// Memorize what endpoints exist in load balancer at the moment
	existing := map[string]endpoint.Endpoint{}
	for _, e := range rr.GetEndpoints() {
		existing[e.GetId()] = e
	}

	// First, add endpoints, that should be added and are not in lb
	for eid, e := range endpointsToAdd {
		if _, exists := existing[eid]; !exists {
			if err := rr.AddEndpoint(e); err != nil {
				log.Errorf("Failed to add %s, err: %s", e, err)
			} else {
				log.Infof("Added %s", e)
			}
		}
	}

	// Second, remove endpoints that should not be there any more
	for eid, e := range existing {
		if _, exists := endpointsToAdd[eid]; !exists {
			if err := rr.RemoveEndpoint(e); err != nil {
				log.Errorf("Failed to remove %s, err: %s", e, err)
			} else {
				log.Infof("Removed %s", e)
			}
		}
	}
	return nil
}

func (s *Service) watchChanges() {
	for {
		change := <-s.changes
		log.Infof("Service got change: %s", change)
		s.processChange(change)
	}
}

func (s *Service) processChange(change *Change) {
	var err error
	switch child := (change.Child).(type) {
	case *Endpoint:
		switch change.Action {
		case "create":
			err = s.addEndpoint((change.Parent).(*Upstream), child)
		case "delete":
			err = s.deleteEndpoint((change.Parent).(*Upstream), child)
		}
	case *Location:
		switch change.Action {
		case "create":
			err = s.addLocation((change.Parent).(*Host), child)
		case "delete":
			err = s.deleteLocation((change.Parent).(*Host), child)
		case "set":
			if len(change.Keys["upstream"]) != 0 {
				err = s.updateLocationUpstream((change.Parent).(*Host), child, change.Keys["upstream"])
			} else {
				err = fmt.Errorf("Unknown property update: %s", change)
			}
		}
	case *Host:
		switch change.Action {
		case "create":
			err = s.addHost(child)
		case "delete":
			err = s.deleteHost(child)
		}
	}
	if err != nil {
		log.Errorf("Processing change failed: %s", err)
	}
}

func (s *Service) getPathRouter(hostname string) (*pathroute.PathRouter, error) {
	r := s.router.GetRouter(hostname)
	if r == nil {
		return nil, fmt.Errorf("Location with host %s not found.", hostname)
	}
	router, ok := r.(*pathroute.PathRouter)
	if !ok {
		return nil, fmt.Errorf("Unknown router type: %T", r)
	}
	return router, nil
}

func (s *Service) getHttpLocation(hostname string, locationId string) (*httploc.HttpLocation, error) {
	router, err := s.getPathRouter(hostname)
	if err != nil {
		return nil, err
	}
	ilo := router.GetLocationById(locationId)
	if ilo == nil {
		return nil, fmt.Errorf("Failed to get location by id: %s", locationId)
	}
	loc, ok := ilo.(*httploc.HttpLocation)
	if !ok {
		return nil, fmt.Errorf("Unsupported location type: %T", ilo)
	}
	return loc, nil
}

func (s *Service) getHttpLocationLb(hostname string, locationId string) (*roundrobin.RoundRobin, error) {
	loc, err := s.getHttpLocation(hostname, locationId)
	if err != nil {
		return nil, err
	}
	rr, ok := loc.GetLoadBalancer().(*roundrobin.RoundRobin)
	if !ok {
		return nil, fmt.Errorf("Unexpected load balancer type: %T", loc.GetLoadBalancer())
	}
	return rr, nil
}

// Returns active locations using given upstream
func (s *Service) getLocations(upstreamId string) ([]*httploc.HttpLocation, error) {
	out := []*httploc.HttpLocation{}

	hosts, err := s.backend.GetHosts()
	if err != nil {
		return nil, fmt.Errorf("Failed to get hosts: %s", hosts)
	}
	for _, h := range hosts {
		for _, l := range h.Locations {
			if l.Upstream.Name != upstreamId {
				continue
			}
			loc, err := s.getHttpLocation(h.Name, l.Name)
			if err != nil {
				return nil, err
			}
			out = append(out, loc)
		}
	}
	return out, nil
}

func (s *Service) addEndpoint(upstream *Upstream, e *Endpoint) error {
	endpoint, err := EndpointFromUrl(e.Name, e.Url)
	if err != nil {
		return fmt.Errorf("Failed to parse endpoint url: %s", endpoint)
	}
	locations, err := s.getLocations(upstream.Name)
	if err != nil {
		return err
	}
	for _, l := range locations {
		rr, ok := l.GetLoadBalancer().(*roundrobin.RoundRobin)
		if !ok {
			return fmt.Errorf("Unexpected load balancer type: %T", l.GetLoadBalancer())
		}
		if err := rr.AddEndpoint(endpoint); err != nil {
			log.Errorf("Failed to add %s, err: %s", e, err)
		} else {
			log.Infof("Added %s", e)
		}
	}
	return nil
}

func (s *Service) deleteEndpoint(upstream *Upstream, e *Endpoint) error {
	endpoint, err := EndpointFromUrl(e.Name, "http://delete.me:4000")
	if err != nil {
		return fmt.Errorf("Failed to parse endpoint url: %s", endpoint)
	}
	locations, err := s.getLocations(upstream.Name)
	if err != nil {
		return err
	}
	for _, l := range locations {
		rr, ok := l.GetLoadBalancer().(*roundrobin.RoundRobin)
		if !ok {
			return fmt.Errorf("Unexpected load balancer type: %T", l.GetLoadBalancer())
		}
		if err := rr.RemoveEndpoint(endpoint); err != nil {
			log.Errorf("Failed to remove endpoint: %s", err)
		} else {
			log.Infof("Removed %s", e)
		}
	}
	return nil
}

func (s *Service) addLocation(host *Host, loc *Location) error {
	router, err := s.getPathRouter(host.Name)
	if err != nil {
		return err
	}
	// Create a load balancer that handles all the endpoints within the given location
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return err
	}

	before := callback.NewBeforeChain()
	after := callback.NewAfterChain()
	options := httploc.Options{
		Before: before,
		After:  after,
	}
	// Add rate limits
	for _, rl := range loc.RateLimits {
		limiter, err := s.newRateLimiter(rl)
		if err == nil {
			before.Add(rl.EtcdKey, limiter)
			after.Add(rl.EtcdKey, limiter)
		} else {
			log.Errorf("Failed to create limiter: %s", before)
		}
	}

	// Add connection limits
	for _, cl := range loc.ConnLimits {
		limiter, err := s.newConnLimiter(cl)
		if err == nil {
			before.Add(cl.EtcdKey, limiter)
			after.Add(cl.EtcdKey, limiter)
		} else {
			log.Errorf("Failed to create limiter: %s", before)
		}
	}

	// Create a location itself
	location, err := httploc.NewLocationWithOptions(loc.Name, rr, options)
	if err != nil {
		return err
	}
	// Add the location to the router
	if err := router.AddLocation(loc.Path, location); err != nil {
		return err
	}
	// Once the location added, configure all endpoints
	return s.configureLocation(host, loc)
}

func (s *Service) newRateLimiter(rl *RateLimit) (*tokenbucket.TokenLimiter, error) {
	mapper, err := VariableToMapper(rl.Variable)
	if err != nil {
		return nil, err
	}
	rate := tokenbucket.Rate{Units: int64(rl.Requests), Period: time.Second * time.Duration(rl.PeriodSeconds)}
	return tokenbucket.NewTokenLimiterWithOptions(mapper, rate, tokenbucket.Options{Burst: rl.Burst})
}

func (s *Service) newConnLimiter(cl *ConnLimit) (*connlimit.ConnectionLimiter, error) {
	mapper, err := VariableToMapper(cl.Variable)
	if err != nil {
		return nil, err
	}
	return connlimit.NewConnectionLimiter(mapper, cl.Connections)
}

func (s *Service) updateLocationUpstream(host *Host, loc *Location, upstreamId string) error {
	return s.configureLocation(host, loc)
}

func (s *Service) deleteLocation(host *Host, loc *Location) error {
	router, err := s.getPathRouter(host.Name)
	if err != nil {
		return err
	}
	location := router.GetLocationById(loc.Name)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
	}
	err = router.RemoveLocation(location)
	if err == nil {
		log.Infof("Removed %s", loc)
	}
	return err
}

func (s *Service) addHost(host *Host) error {
	router := pathroute.NewPathRouter()
	return s.router.SetRouter(host.Name, router)
}

func (s *Service) deleteHost(host *Host) error {
	s.router.RemoveRouter(host.Name)
	log.Infof("Removed %s", host)
	return nil
}

func (s *Service) startProxy() error {
	addr := fmt.Sprintf("%s:%d", s.options.Interface, s.options.Port)
	server := &http.Server{
		Addr:           addr,
		Handler:        s.proxy,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}

func (s *Service) startApi() error {
	addr := fmt.Sprintf("%s:%d", s.options.ApiInterface, s.options.ApiPort)

	server := &http.Server{
		Addr:           addr,
		Handler:        s.apiRouter,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}

type VulcanEndpoint struct {
	Url *url.URL
	Id  string
}

func EndpointFromUrl(id string, u string) (*VulcanEndpoint, error) {
	url, err := netutils.ParseUrl(u)
	if err != nil {
		return nil, err
	}
	return &VulcanEndpoint{Url: url, Id: id}, nil
}

func (e *VulcanEndpoint) String() string {
	return fmt.Sprintf("endpoint(id=%s, url=%s)", e.Id, e.Url.String())
}

func (e *VulcanEndpoint) GetId() string {
	return e.Id
}

func (e *VulcanEndpoint) GetUrl() *url.URL {
	return e.Url
}
