package service

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	runtime "github.com/mailgun/gotools-runtime"
	"github.com/mailgun/vulcan"
	//"github.com/mailgun/vulcan/loadbalance/roundrobin"
	//"github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcan/route/hostroute"
	//"github.com/mailgun/vulcan/route/pathroute"
	"net/http"
	"time"
)

type Service struct {
	client  *etcd.Client
	proxy   *vulcan.Proxy
	options Options
	router  *hostroute.HostRouter
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
		client:  etcd.NewClient(options.EtcdNodes),
	}
}

func (s *Service) Start() error {
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

	return s.startProxy()
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

func (s *Service) configureProxy() error {
	response, err := s.client.Get(s.options.EtcdKey, true, false)
	if err != nil {
		return err
	}
	fmt.Printf("Response: %s", response)
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
