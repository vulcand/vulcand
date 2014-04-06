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
	. "github.com/mailgun/vulcand/backend"
	"net/http"
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
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
	}
}

func (s *Service) Start() error {
	// Init logging
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	backend, err := NewEtcdBackend(s.options.EtcdNodes, s.options.EtcdKey, s.options.EtcdConsistency)
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

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	log.Infof("Got signal %s, exiting now", <-c)
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
	api.InitProxyController(s.backend, s.apiRouter)
	return nil
}

func (s *Service) configureProxy() error {
	hosts, err := s.backend.GetHosts()
	if err != nil {
		return err
	}
	for _, host := range hosts {
		log.Infof("Configuring host: %s", host.Name)
		// Create a router that will route the request within the given server name
		router := pathroute.NewPathRouter()
		if err := s.configureHost(router, host); err != nil {
			log.Errorf("Failed configuring host: %s", host.Name)
			return err
		}
		// Add the path regex router to the parent hostname based router
		s.router.SetRouter(host.Name, router)
	}
	return nil
}

func (s *Service) configureHost(router *pathroute.PathRouter, host *Host) error {
	for _, loc := range host.Locations {
		// Create a load balancer that handles all the endpoints within the given location
		rr, err := roundrobin.NewRoundRobin()
		if err != nil {
			return err
		}
		if err := s.configureEndpoints(rr, loc.Upstream); err != nil {
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
		log.Infof("Added Location %s to Host: %s", loc.Path, host.Name)
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
			log.Errorf("Ignoring endpoint: %s", err)
			continue
		}
		log.Infof("Added Endpoint(%s)", e.Url)
	}
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
