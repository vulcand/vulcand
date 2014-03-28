package service

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	runtime "github.com/mailgun/gotools-runtime"
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcan/route"
	"net/http"
	"time"
)

type Service struct {
	client  *etcd.Client
	proxy   *vulcan.ReverseProxy
	options Options
}

func NewService(options Options) *Service {
	return &Service{
		options: options,
		client:  etcd.NewClient(options.EtcdNodes),
	}
}

func (s *Service) Start() error {
	if s.options.PidPath != "" {
		if err := runtime.WritePid(s.options.PidPath); err != nil {
			return fmt.Errorf("Failed to write PID file: %v\n", err)
		}
	}

	var err error
	if s.proxy, err = s.newProxy(); err != nil {
		return err
	}

	return s.startProxy()
}

func (s *Service) newProxy() (*vulcan.ReverseProxy, error) {
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return nil, err
	}
	location, err := location.NewHttpLocation(
		location.HttpLocationSettings{LoadBalancer: rr})
	if err != nil {
		return nil, err
	}
	proxySettings := vulcan.ProxySettings{
		Router: &route.MatchAll{
			Location: location,
		},
	}
	return vulcan.NewReverseProxy(proxySettings)
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
