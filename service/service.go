package service

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/gorilla/mux"
	log "github.com/mailgun/gotools-log"
	runtime "github.com/mailgun/gotools-runtime"
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcan/route/hostroute"
	"github.com/mailgun/vulcan/route/pathroute"
	"github.com/mailgun/vulcand/api"
	. "github.com/mailgun/vulcand/proxy"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type Service struct {
	client    *etcd.Client
	proxy     *vulcan.Proxy
	options   Options
	router    *hostroute.HostRouter
	apiRouter *mux.Router
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
		client:  etcd.NewClient(options.EtcdNodes),
	}
}

func (s *Service) Start() error {
	// Init logging
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	if err := s.client.SetConsistency(s.options.EtcdConsistency); err != nil {
		return nil
	}

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

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	log.Infof("Got signal %s, exiting now", <-c)
	return nil
}

func (s *Service) GetServers() ([]Server, error) {
	return s.readServers()
}

func (s *Service) AddServer(name string) error {
	return nil
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
	api.InitServerController(s, s.apiRouter)
	return nil
}

func (s *Service) configureProxy() error {
	servers, err := s.readServers()
	if err != nil {
		return err
	}
	for _, server := range servers {
		log.Infof("Configuring server: %s", server.Name)
		// Create a router that will route the request within the given server name
		router := pathroute.NewPathRouter()
		if err := s.configureServer(router, &server); err != nil {
			log.Errorf("Failed configuring server: %s", server.Name)
			return err
		}
		// Add the path regex router to the parent hostname based router
		s.router.SetRouter(server.Name, router)
	}
	return nil
}

func (s *Service) configureServer(router *pathroute.PathRouter, server *Server) error {
	for _, loc := range server.Locations {
		// Create a load balancer that handles all the endpoints within the given location
		rr, err := roundrobin.NewRoundRobin()
		if err != nil {
			return err
		}
		if err := s.configureEndpoints(rr, &loc.Upstream); err != nil {
			return err
		}

		// Create a location itself
		location, err := httploc.NewLocation(rr)
		if err != nil {
			return err
		}
		// Add the location to the router
		if err := router.AddLocation(loc.Path, location); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) configureEndpoints(rr *roundrobin.RoundRobin, upstream *Upstream) error {
	// Add all endpoints from the upstream to the router
	for _, e := range upstream.Endpoints {
		endpoint, err := endpoint.ParseUrl(e.Url)
		if err != nil {
			log.Errorf("Ignoring endpoint: failed to parse url: %s", endpoint)
			continue
		}
		if err := rr.AddEndpoint(endpoint); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) readServers() ([]Server, error) {
	var servers []Server
	for _, serverKey := range s.getDirs(s.options.EtcdKey, "servers") {
		server := Server{
			EtcdKey: serverKey,
			Name:    suffix(serverKey),
		}
		if err := s.readServerLocations(&server); err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, nil
}

func (s *Service) readServerLocations(server *Server) error {
	locationKeys := s.getDirs(server.EtcdKey, "locations")
	for _, locationKey := range locationKeys {
		path, ok := s.getVal(locationKey, "path")
		if !ok {
			log.Errorf("Missing location path: %s", locationKey)
			continue
		}
		upstreamKey, ok := s.getVal(locationKey, "upstream")
		if !ok {
			log.Errorf("Missing location upstream key: %s", locationKey)
			continue
		}
		location := Location{
			EtcdKey:  suffix(locationKey),
			Path:     path,
			Upstream: Upstream{EtcdKey: upstreamKey},
		}
		if err := s.readLocationUpstream(&location); err != nil {
			log.Errorf("Failed to read location upstream: %s", err)
		}
		server.Locations = append(server.Locations, location)
	}
	return nil
}

func (s *Service) readLocationUpstream(location *Location) error {
	endpointPairs := s.getVals(location.Upstream.EtcdKey, "endpoints")
	log.Infof("Location(%s) endpoints(%s)", location.EtcdKey, endpointPairs)
	for _, e := range endpointPairs {
		_, err := endpoint.ParseUrl(e.Val)
		if err != nil {
			fmt.Printf("Ignoring endpoint: failed to parse url: %s", e.Val)
			continue
		}
		location.Upstream.Endpoints = append(location.Upstream.Endpoints, Endpoint{Url: e.Val, EtcdKey: e.Key})
	}
	return nil
}

func (s *Service) getVal(keys ...string) (string, bool) {
	response, err := s.client.Get(strings.Join(keys, "/"), false, false)
	if notFound(err) {
		return "", false
	}
	if isDir(response.Node) {
		return "", false
	}
	return response.Node.Value, true
}

func (s *Service) getDirs(keys ...string) []string {
	var out []string
	response, err := s.client.Get(strings.Join(keys, "/"), true, true)
	if notFound(err) {
		return out
	}

	if !isDir(response.Node) {
		return out
	}

	for _, srvNode := range response.Node.Nodes {
		if isDir(&srvNode) {
			out = append(out, srvNode.Key)
		}
	}
	return out
}

func (s *Service) getVals(keys ...string) []Pair {
	var out []Pair
	response, err := s.client.Get(strings.Join(keys, "/"), true, true)
	if notFound(err) {
		return out
	}

	if !isDir(response.Node) {
		return out
	}

	for _, srvNode := range response.Node.Nodes {
		if !isDir(&srvNode) {
			out = append(out, Pair{srvNode.Key, srvNode.Value})
		}
	}
	return out
}

type Pair struct {
	Key string
	Val string
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

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[len(vals)-1]
}

func notFound(err error) bool {
	if err == nil {
		return false
	}
	eErr, ok := err.(*etcd.EtcdError)
	return ok && eErr.ErrorCode == 100
}

func isDir(n *etcd.Node) bool {
	return n != nil && n.Dir == true
}
