package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/mailgun/manners"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route/hostroute"
	"github.com/mailgun/vulcand/backend"
)

// server contains all what's necessary to run the HTTP(s) server. server does not work on it's own,
// it heavilly dependes on MuxServer and it acts as is it's internal data structure.
type server struct {
	defaultHost string
	mux         *MuxServer
	router      *hostroute.HostRouter
	srv         *manners.GracefulServer
	proxy       *vulcan.Proxy
	listener    backend.Listener
	listeners   map[string]backend.Listener
	certs       map[string]*backend.Certificate
	options     Options
	state       int
}

func (s *server) String() string {
	return fmt.Sprintf("%s->srv(%s, %s)", s.mux, srvStateDescription(s.state), s.listener.String())
}

func newServer(m *MuxServer, host *backend.Host, r route.Router, l *backend.Listener) (*server, error) {
	certs := make(map[string]*backend.Certificate)
	if host.Cert != nil {
		certs[host.Name] = host.Cert
	}

	router := hostroute.NewHostRouter()
	proxy, err := vulcan.NewProxy(router)
	if err != nil {
		return nil, err
	}

	if err := router.SetRouter(host.Name, r); err != nil {
		return nil, err
	}

	defaultHost := ""
	if host.Options.Default {
		defaultHost = host.Name
	}

	return &server{
		mux:         m,
		listeners:   map[string]backend.Listener{host.Name: *l},
		router:      router,
		proxy:       proxy,
		listener:    *l,
		defaultHost: defaultHost,
		certs:       certs,
		state:       srvStateInit,
	}, nil
}

func (s *server) deleteHost(hostname string) (bool, error) {
	if s.router.GetRouter(hostname) == nil {
		return false, fmt.Errorf("host %s not found", hostname)
	}
	s.router.RemoveRouter(hostname)
	delete(s.listeners, hostname)

	if len(s.listeners) == 0 {
		s.srv.Close()
		return true, nil
	}

	if _, exists := s.certs[hostname]; exists {
		delete(s.certs, hostname)
		if s.defaultHost == hostname {
			s.defaultHost = ""
		}
		return false, s.reload()
	}
	return false, nil
}

func (srv *server) isTLS() bool {
	return srv.listener.Protocol == backend.HTTPS
}

func (s *server) updateHostCert(hostname string, cert *backend.Certificate) error {
	old, exists := s.certs[hostname]
	if !exists {
		return fmt.Errorf("host %s certificate not found")
	}
	if old.Equals(cert) {
		return nil
	}
	s.certs[hostname] = cert
	return s.reload()
}

func (s *server) addHost(host *backend.Host, router route.Router, listener *backend.Listener) error {
	if s.router.GetRouter(host.Name) != nil {
		return fmt.Errorf("host %s already registered", host)
	}

	if l, exists := s.listeners[host.Name]; exists {
		return fmt.Errorf("host %s arlready has a registered listener %s", host, l)
	}

	s.listeners[host.Name] = *listener

	if err := s.router.SetRouter(host.Name, router); err != nil {
		return err
	}

	if host.Options.Default {
		s.defaultHost = host.Name
	}

	// We are serving TLS, reload server
	if host.Cert != nil {
		s.certs[host.Name] = host.Cert
		return s.reload()
	}
	return nil
}

func (s *server) isServing() bool {
	return s.state == srvStateActive
}

func (s *server) hasListeners() bool {
	return s.state == srvStateActive || s.state == srvStateHijacked
}

func (s *server) hijackListenerFrom(so *server) error {
	log.Infof("%s hijackListenerFrom %s", s, so)
	// in case if the TLS in not served, we dont' need to do anything as it's all done by the proxy
	var config *tls.Config
	if s.isTLS() {
		var err error
		config, err = newTLSConfig(s.certs, s.defaultHost)
		if err != nil {
			return err
		}
	}

	gracefulServer, err := so.srv.HijackListener(s.newHTTPServer(), config)
	if err != nil {
		return err
	}
	s.srv = gracefulServer
	s.state = srvStateHijacked
	return nil
}

func (s *server) newHTTPServer() *http.Server {
	return &http.Server{
		Handler:        s.proxy,
		ReadTimeout:    s.options.ReadTimeout,
		WriteTimeout:   s.options.WriteTimeout,
		MaxHeaderBytes: s.options.MaxHeaderBytes,
	}
}

func (s *server) reload() error {
	if !s.isServing() {
		return nil
	}

	// in case if the TLS in not served, we dont' need to do anything as it's all done by the proxy
	if !s.isTLS() {
		return nil
	}

	// Otherwise, we need to generate new TLS config and spin up the new server on the same socket
	config, err := newTLSConfig(s.certs, s.defaultHost)
	if err != nil {
		return err
	}
	gracefulServer, err := s.srv.HijackListener(s.newHTTPServer(), config)
	if err != nil {
		return err
	}
	go s.serve(gracefulServer, nil)

	s.srv.Close()
	s.srv = gracefulServer
	return nil
}

func (s *server) shutdown() {
	if s.srv != nil {
		s.srv.Close()
	}
}

func (s *server) hasListener(hostname, listenerId string) bool {
	l, exists := s.listeners[hostname]
	return exists && l.Id == listenerId
}

func (s *server) hasHost(hostname string) bool {
	_, exists := s.listeners[hostname]
	return exists
}

func newTLSConfig(certs map[string]*backend.Certificate, defaultHost string) (*tls.Config, error) {
	config := &tls.Config{}

	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	pairs := make(map[string]tls.Certificate, len(certs))
	for h, c := range certs {
		cert, err := tls.X509KeyPair(c.Cert, c.Key)
		if err != nil {
			return nil, err
		}
		pairs[h] = cert
	}

	config.Certificates = make([]tls.Certificate, 0, len(certs))
	if defaultHost != "" {
		cert, exists := pairs[defaultHost]
		if !exists {
			return nil, fmt.Errorf("default host '%s' certificate is not passed", defaultHost)
		}
		config.Certificates = append(config.Certificates, cert)
	}

	for h, cert := range pairs {
		if h != defaultHost {
			config.Certificates = append(config.Certificates, cert)
		}
	}

	config.BuildNameToCertificate()
	return config, nil
}

func (s *server) start() error {
	log.Infof("%s start", s)
	switch s.state {
	case srvStateInit:
		listener, err := net.Listen(s.listener.Address.Network, s.listener.Address.Address)
		if err != nil {
			return err
		}

		if s.isTLS() {
			config, err := newTLSConfig(s.certs, s.defaultHost)
			if err != nil {
				return err
			}
			listener = manners.NewTLSListener(manners.TCPKeepAliveListener{listener.(*net.TCPListener)}, config)
		}
		s.srv = manners.NewWithServer(s.newHTTPServer())
		s.state = srvStateActive
		go s.serve(s.srv, listener)
		return nil
	case srvStateHijacked:
		s.state = srvStateActive
		go s.serve(s.srv, nil)
		return nil
	}
	return fmt.Errorf("%s Calling start in unsupported state: %d", s.state)
}

func (s *server) serve(srv *manners.GracefulServer, listener net.Listener) {
	log.Infof("%s serve", s)

	s.mux.wg.Add(1)
	defer s.mux.wg.Done()

	if listener == nil {
		srv.ListenAndServe()
	} else {
		s.srv.Serve(listener)
	}

	log.Infof("Stop %s", s.listener.String())
}

const (
	srvStateInit     = iota // server has been created
	srvStateActive   = iota // server is active and is serving requests
	srvStateHijacked = iota // server has hijacked listeners from other server
)

func srvStateDescription(state int) string {
	switch state {
	case srvStateInit:
		return "init"
	case srvStateActive:
		return "active"
	case srvStateHijacked:
		return "hijacked"
	}
	return "undefined"
}
