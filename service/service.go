package service

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/mailgun/go-etcd/etcd"
	log "github.com/mailgun/gotools-log"
	runtime "github.com/mailgun/gotools-runtime"
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/route/hostroute"
	"github.com/mailgun/vulcand/adapter"
	"github.com/mailgun/vulcand/api"
	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/backend/etcdbackend"
	. "github.com/mailgun/vulcand/configure"
	"net/http"
	"os"
	"os/signal"
	"time"
)

type Service struct {
	client       *etcd.Client
	proxy        *vulcan.Proxy
	backend      Backend
	options      Options
	router       *hostroute.HostRouter
	apiRouter    *mux.Router
	changes      chan interface{}
	configurator *Configurator
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
		changes: make(chan interface{}),
	}
}

func (s *Service) Start() error {
	// Init logging
	log.Init([]*log.LogConfig{&log.LogConfig{Name: s.options.Log}})

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
	go s.startProxy()

	s.configurator = NewConfigurator(s.proxy)

	// Tell backend to watch configuration changes and pass them to the channel
	// the second parameter tells backend to do the initial read of the configuration
	// and produce the stream of changes so proxy would initialise initial config
	go s.backend.WatchChanges(s.changes, true)
	// Configurator will listen to the changes from the channel and will
	go s.configurator.WatchChanges(s.changes)

	if err := s.initApi(); err != nil {
		return err
	}

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

func (s *Service) initApi() error {
	s.apiRouter = mux.NewRouter()
	api.InitProxyController(s.backend, adapter.NewAdapter(s.proxy), s.configurator.GetConnWatcher(), s.apiRouter)
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
