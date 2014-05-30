package service

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/gorilla/mux"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	runtime "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-runtime"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route/hostroute"
	"github.com/mailgun/vulcand/adapter"
	"github.com/mailgun/vulcand/api"
	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/backend/etcdbackend"
	. "github.com/mailgun/vulcand/configure"
	"github.com/mailgun/vulcand/plugin/registry"
	"net/http"
	"os"
	"os/signal"
)

type Service struct {
	client       *etcd.Client
	proxy        *vulcan.Proxy
	backend      Backend
	options      Options
	router       *hostroute.HostRouter
	apiRouter    *mux.Router
	errorC       chan error
	changeC      chan interface{}
	sigC         chan os.Signal
	configurator *Configurator
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
		changeC: make(chan interface{}),
		errorC:  make(chan error),
		sigC:    make(chan os.Signal),
	}
}

func (s *Service) Start() error {
	// Init logging
	log.Init([]*log.LogConfig{&log.LogConfig{Name: s.options.Log}})

	backend, err := NewEtcdBackend(registry.GetRegistry(), s.options.EtcdNodes, s.options.EtcdKey, s.options.EtcdConsistency)
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
	go s.watchChanges()

	if err := s.initApi(); err != nil {
		return err
	}

	go s.startApi()
	signal.Notify(s.sigC, os.Interrupt, os.Kill)

	// Block until a signal is received or we got an error
	select {
	case signal := <-s.sigC:
		log.Infof("Got signal %s, exiting now", signal)
		return nil
	case err := <-s.errorC:
		log.Infof("Got request to shutdown with error: %s", err)
		return err
	}
	return nil
}

func (s *Service) watchChanges() {
	go s.configurator.WatchChanges(s.changeC)
	err := s.backend.WatchChanges(s.changeC, true)
	if err != nil {
		log.Infof("Stopped watching changes with error: %s. Shutting down with error", err)
		s.errorC <- err
	} else {
		log.Infof("Stopped watching changes without error. Will continue running", err)
	}
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
		ReadTimeout:    s.options.ReadTimeout,
		WriteTimeout:   s.options.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}

func (s *Service) startApi() error {
	addr := fmt.Sprintf("%s:%d", s.options.ApiInterface, s.options.ApiPort)

	server := &http.Server{
		Addr:           addr,
		Handler:        s.apiRouter,
		ReadTimeout:    s.options.ReadTimeout,
		WriteTimeout:   s.options.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}
