package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/manners"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route/hostroute"
	"github.com/mailgun/vulcand/backend"
)

// server contains all what's necessary to run the HTTP(s) server. server does not work on it's own,
// it heavilly depends on MuxServer and acts as is it's internal data structure.
type server struct {
	defaultHost string
	mux         *MuxServer
	router      *hostroute.HostRouter
	srv         *manners.GracefulServer
	proxy       *vulcan.Proxy
	listener    backend.Listener
	listeners   map[string]backend.Listener
	keyPairs    map[string]*backend.KeyPair
	options     Options
	state       int
}

func (s *server) GetFile() (*FileDescriptor, error) {
	if !s.hasListeners() || s.srv == nil {
		return nil, nil
	}
	file, err := s.srv.GetFile()
	if err != nil {
		return nil, err
	}
	return &FileDescriptor{
		File:    file,
		Address: s.listener.Address,
	}, nil
}

func (s *server) String() string {
	return fmt.Sprintf("%s->srv(%v, %v)", s.mux, s.state, s.listener)
}

func newServer(m *MuxServer, host *backend.Host, r route.Router, l *backend.Listener) (*server, error) {
	keyPairs := make(map[string]*backend.KeyPair)
	if host.KeyPair != nil {
		keyPairs[host.Name] = host.KeyPair
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
		keyPairs:    keyPairs,
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
		s.shutdown()
		return true, nil
	}

	if _, exists := s.keyPairs[hostname]; exists {
		delete(s.keyPairs, hostname)
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

func (s *server) updateHostKeyPair(hostname string, keyPair *backend.KeyPair) error {
	old, exists := s.keyPairs[hostname]
	if !exists {
		return fmt.Errorf("host %s keyPairificate not found")
	}
	if old.Equals(keyPair) {
		return nil
	}
	s.keyPairs[hostname] = keyPair
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
	if host.KeyPair != nil {
		s.keyPairs[host.Name] = host.KeyPair
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

func (s *server) takeFile(f *FileDescriptor) error {
	log.Infof("%s takeFile %d", s, f)

	listener, err := f.ToListener()
	if err != nil {
		return err
	}

	if s.isTLS() {
		tcpListener, ok := listener.(*net.TCPListener)
		if !ok {
			return fmt.Errorf("%s can not take file for TLS listener when asked to take type %T from %s", s, listener, f)
		}
		config, err := newTLSConfig(s.keyPairs, s.defaultHost)
		if err != nil {
			return err
		}
		listener = manners.NewTLSListener(
			manners.TCPKeepAliveListener{tcpListener}, config)
	}

	s.srv = manners.NewWithOptions(
		manners.Options{
			Server:       s.newHTTPServer(),
			Listener:     listener,
			StateHandler: s.mux.connTracker.onStateChange,
		})
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
	config, err := newTLSConfig(s.keyPairs, s.defaultHost)
	if err != nil {
		return err
	}
	gracefulServer, err := s.srv.HijackListener(s.newHTTPServer(), config)
	if err != nil {
		return err
	}
	go s.serve(gracefulServer)

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

func newTLSConfig(keyPairs map[string]*backend.KeyPair, defaultHost string) (*tls.Config, error) {
	config := &tls.Config{}

	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	pairs := make(map[string]tls.Certificate, len(keyPairs))
	for h, c := range keyPairs {
		keyPair, err := tls.X509KeyPair(c.Cert, c.Key)
		if err != nil {
			return nil, err
		}
		pairs[h] = keyPair
	}

	config.Certificates = make([]tls.Certificate, 0, len(keyPairs))
	if defaultHost != "" {
		keyPair, exists := pairs[defaultHost]
		if !exists {
			return nil, fmt.Errorf("default host '%s' certificate is not passed", defaultHost)
		}
		config.Certificates = append(config.Certificates, keyPair)
	}

	for h, keyPair := range pairs {
		if h != defaultHost {
			config.Certificates = append(config.Certificates, keyPair)
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
			config, err := newTLSConfig(s.keyPairs, s.defaultHost)
			if err != nil {
				return err
			}
			listener = manners.NewTLSListener(
				manners.TCPKeepAliveListener{listener.(*net.TCPListener)}, config)
		}
		s.srv = manners.NewWithOptions(
			manners.Options{
				Server:       s.newHTTPServer(),
				Listener:     listener,
				StateHandler: s.mux.connTracker.onStateChange,
			})
		s.state = srvStateActive
		go s.serve(s.srv)
		return nil
	case srvStateHijacked:
		s.state = srvStateActive
		go s.serve(s.srv)
		return nil
	}
	return fmt.Errorf("%v Calling start in unsupported state: %d", s.state)
}

func (s *server) serve(srv *manners.GracefulServer) {
	log.Infof("%s serve", s)

	s.mux.wg.Add(1)
	defer s.mux.wg.Done()

	srv.ListenAndServe()

	log.Infof("Stop %s", s.listener.String())
}

type srvState int

const (
	srvStateInit     = iota // server has been created
	srvStateActive   = iota // server is active and is serving requests
	srvStateHijacked = iota // server has hijacked listeners from other server
)

func (s srvState) String() string {
	switch s {
	case srvStateInit:
		return "init"
	case srvStateActive:
		return "active"
	case srvStateHijacked:
		return "hijacked"
	}
	return "undefined"
}
