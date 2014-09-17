package service

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/api"
	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/backend/etcdbackend"
	"github.com/mailgun/vulcand/connwatch"
	"github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/secret"
	"github.com/mailgun/vulcand/server"
	"github.com/mailgun/vulcand/supervisor"
)

func Run(registry *plugin.Registry) error {
	options, err := ParseCommandLine()
	if err != nil {
		return fmt.Errorf("failed to parse command line: %s", err)
	}
	service := NewService(options, registry)
	if err := service.Start(); err != nil {
		return fmt.Errorf("service start failure: %s", err)
	} else {
		log.Infof("Service exited gracefully")
	}
	return nil
}

type Service struct {
	client        *etcd.Client
	options       Options
	registry      *plugin.Registry
	apiApp        *scroll.App
	errorC        chan error
	sigC          chan os.Signal
	supervisor    *supervisor.Supervisor
	metricsClient metrics.Client
}

func NewService(options Options, registry *plugin.Registry) *Service {
	return &Service{
		registry: registry,
		options:  options,
		errorC:   make(chan error),
		sigC:     make(chan os.Signal),
	}
}

func (s *Service) Start() error {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: s.options.Log}})

	if s.options.PidPath != "" {
		ioutil.WriteFile(s.options.PidPath, []byte(fmt.Sprint(os.Getpid())), 0644)
	}

	if s.options.StatsdAddr != "" {
		var err error
		s.metricsClient, err = metrics.NewStatsd(s.options.StatsdAddr, s.options.StatsdPrefix)
		if err != nil {
			return err
		}
	}

	s.supervisor = supervisor.NewSupervisor(s.newServer, s.newBackend, s.errorC)

	// Tells configurator to perform initial proxy configuration and start watching changes
	if err := s.supervisor.Start(); err != nil {
		return err
	}

	if err := s.initApi(); err != nil {
		return err
	}

	go func() {
		s.errorC <- s.startApi()
	}()

	if s.metricsClient != nil {
		go s.reportSystemMetrics()
	}
	signal.Notify(s.sigC, os.Interrupt, os.Kill, syscall.SIGTERM)

	// Block until a signal is received or we got an error
	select {
	case signal := <-s.sigC:
		if signal == syscall.SIGTERM {
			log.Infof("Got signal %s, shutting down gracefully", signal)
			s.supervisor.Stop(true)
			log.Infof("All servers stopped")
		} else {
			log.Infof("Got signal %s, exiting now without waiting", signal)
			s.supervisor.Stop(false)
		}
		return nil
	case err := <-s.errorC:
		log.Infof("Got request to shutdown with error: %s", err)
		return err
	}
	return nil
}

func (s *Service) newBox() (*secret.Box, error) {
	if s.options.SealKey == "" {
		return nil, nil
	}
	key, err := secret.KeyFromString(s.options.SealKey)
	if err != nil {
		return nil, err
	}
	return secret.NewBox(key)
}

func (s *Service) newBackend() (backend.Backend, error) {
	box, err := s.newBox()
	if err != nil {
		return nil, err
	}
	return etcdbackend.NewEtcdBackendWithOptions(
		s.registry, s.options.EtcdNodes, s.options.EtcdKey,
		etcdbackend.Options{
			EtcdConsistency: s.options.EtcdConsistency,
			Box:             box,
		})
}

func (s *Service) reportSystemMetrics() {
	for {
		s.metricsClient.ReportRuntimeMetrics("sys", 1.0)
		// we have 256 time buckets for gc stats, GC is being executed every 4ms on average
		// so we have 256 * 4 = 1024 around one second to report it. To play safe, let's report every 300ms
		time.Sleep(300 * time.Millisecond)
	}
}

func (s *Service) newServer(id int, cw *connwatch.ConnectionWatcher) (server.Server, error) {
	return server.NewMuxServerWithOptions(id, cw, server.Options{
		MetricsClient:  s.metricsClient,
		DialTimeout:    s.options.EndpointDialTimeout,
		ReadTimeout:    s.options.ServerReadTimeout,
		WriteTimeout:   s.options.ServerWriteTimeout,
		MaxHeaderBytes: s.options.ServerMaxHeaderBytes,
		DefaultListener: &backend.Listener{
			Id:       "DefaultListener",
			Protocol: "http",
			Address: backend.Address{
				Network: "tcp",
				Address: fmt.Sprintf("%s:%d", s.options.Interface, s.options.Port),
			},
		},
	})
}

func (s *Service) initApi() error {
	s.apiApp = scroll.NewApp()
	b, err := s.newBackend()
	if err != nil {
		return err
	}
	api.InitProxyController(b, s.supervisor, s.supervisor.GetConnWatcher(), s.apiApp)
	return nil
}

func (s *Service) startApi() error {
	addr := fmt.Sprintf("%s:%d", s.options.ApiInterface, s.options.ApiPort)

	server := &http.Server{
		Addr:           addr,
		Handler:        s.apiApp.GetHandler(),
		ReadTimeout:    s.options.ServerReadTimeout,
		WriteTimeout:   s.options.ServerWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	return server.ListenAndServe()
}
