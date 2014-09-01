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

// MuxServer is capable of listening on multiple interfaces, graceful shutdowns and updating TLS certificates
type MuxServer struct {
	// Each listener address has a server associated with it
	servers map[backend.Address]*srvPack

	// Options hold parameters that are used to initialize http servers
	options Options
}

func NewMuxServer() (*MuxServer, error) {
	return nil, nil
}

func NewMuxServerWithOptions(o Options) (*MuxServer, error) {
	return &MuxServer{
		servers: make(map[backend.Address]*srvPack),
		options: o,
	}, nil
}

func (m *MuxServer) AddHostListener(host *backend.Host, router route.Router, l *backend.Listener) error {
	s, exists := m.servers[l.Address]
	if !exists {
		var err error
		if s, err = newSrvPack(host, router, l, m.options); err != nil {
			return err
		}
		m.servers[l.Address] = s
		go s.serve()
		return nil
	}

	// We can not listen for different protocols on the same socket
	if s.listener.Protocol != l.Protocol {
		return fmt.Errorf("conflicting protocol %s and %s", s.listener.Protocol, l.Protocol)
	}

	return s.addHost(host, router, l)
}

func (m *MuxServer) DeleteHostListener(hostname string, listenerId string) error {
	log.Infof("DeleteHostListener %s %s", hostname, listenerId)
	var err error
	for k, s := range m.servers {
		if s.hasListener(hostname, listenerId) {
			closed, e := s.deleteHost(hostname)
			if closed {
				log.Infof("Closed server listening on %s", k)
				delete(m.servers, k)
			}
			err = e
		}
	}
	return err
}

func (m *MuxServer) DeleteHostListeners(hostname string) error {
	var err error
	for _, s := range m.servers {
		_, err = s.deleteHost(hostname)
	}
	return err
}

func (m *MuxServer) HasHostListener(hostname, listenerId string) bool {
	for _, s := range m.servers {
		if s.hasListener(hostname, listenerId) {
			return true
		}
	}
	return false
}

func (m *MuxServer) UpdateHostCert(hostname string, cert *backend.Certificate) error {
	for _, s := range m.servers {
		if s.hasHost(hostname) && s.isTLS() {
			if err := s.updateHostCert(hostname, cert); err != nil {
				return err
			}
		}
	}
	return nil
}

type srvPack struct {
	defaultHost string
	router      *hostroute.HostRouter
	srv         *manners.GracefulServer
	proxy       *vulcan.Proxy
	listener    backend.Listener
	netListener net.Listener
	certs       map[string]*backend.Certificate
	options     Options
	listeners   map[string]backend.Listener
}

func newSrvPack(host *backend.Host, r route.Router, l *backend.Listener, o Options) (*srvPack, error) {
	var err error
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

	listener, err := net.Listen(l.Address.Network, l.Address.Address)
	if err != nil {
		return nil, err
	}

	defaultHost := ""
	if host.Default {
		defaultHost = host.Name
	}

	if len(certs) != 0 {
		config, err := newTLSConfig(certs, defaultHost)
		if err != nil {
			return nil, err
		}
		listener = tls.NewListener(manners.TCPKeepAliveListener{listener.(*net.TCPListener)}, config)
	}

	server := &http.Server{
		Handler:        proxy,
		ReadTimeout:    o.ReadTimeout,
		WriteTimeout:   o.WriteTimeout,
		MaxHeaderBytes: o.MaxHeaderBytes,
	}

	gracefulServer := manners.NewWithServer(server)

	return &srvPack{
		listeners:   map[string]backend.Listener{host.Name: *l},
		router:      router,
		proxy:       proxy,
		srv:         gracefulServer,
		listener:    *l,
		netListener: listener,
		defaultHost: defaultHost,
		certs:       certs,
	}, nil
}

func (srv *srvPack) deleteHost(hostname string) (bool, error) {
	if srv.router.GetRouter(hostname) == nil {
		return false, fmt.Errorf("host %s not found", hostname)
	}
	srv.router.RemoveRouter(hostname)
	delete(srv.listeners, hostname)

	if len(srv.listeners) == 0 {
		srv.srv.Close()
		return true, nil
	}

	if _, exists := srv.certs[hostname]; exists {
		delete(srv.certs, hostname)
		if srv.defaultHost == hostname {
			srv.defaultHost = ""
		}
		return false, srv.reload()
	}
	return false, nil
}

func (srv *srvPack) isTLS() bool {
	return srv.listener.Protocol == backend.HTTPS
}

func (srv *srvPack) updateHostCert(hostname string, cert *backend.Certificate) error {
	old, exists := srv.certs[hostname]
	if !exists {
		return fmt.Errorf("host %s certificate not found")
	}
	if old.Equals(cert) {
		return nil
	}
	srv.certs[hostname] = cert
	return srv.reload()
}

func (srv *srvPack) addHost(host *backend.Host, router route.Router, listener *backend.Listener) error {
	if srv.router.GetRouter(host.Name) != nil {
		return fmt.Errorf("host %s already registered", host)
	}

	if l, exists := srv.listeners[host.Name]; exists {
		return fmt.Errorf("host %s arlready has a registered listener %s", host, l)
	}

	srv.listeners[host.Name] = *listener

	if err := srv.router.SetRouter(host.Name, router); err != nil {
		return err
	}

	if host.Default {
		srv.defaultHost = host.Name
	}

	// We are serving TLS, reload server
	if host.Cert != nil {
		srv.certs[host.Name] = host.Cert
		return srv.reload()
	}
	return nil
}

func (srv *srvPack) reload() error {
	// in case if the TLS in not served, we dont' need to do anything
	if len(srv.certs) == 0 {
		return nil
	}

	// Otherwise, we need to generate new TLS config and spin up the new server on the same socket
	config, err := newTLSConfig(srv.certs, srv.defaultHost)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Handler:        srv.proxy,
		ReadTimeout:    srv.options.ReadTimeout,
		WriteTimeout:   srv.options.WriteTimeout,
		MaxHeaderBytes: srv.options.MaxHeaderBytes,
	}
	gracefulServer, err := srv.srv.HijackListener(httpServer, config)
	if err != nil {
		return err
	}
	go gracefulServer.ListenAndServe()
	srv.srv.Close()
	srv.srv = gracefulServer
	return nil
}

func (srv *srvPack) hasListener(hostname, listenerId string) bool {
	l, exists := srv.listeners[hostname]
	return exists && l.Id == listenerId
}

func (srv *srvPack) hasHost(hostname string) bool {
	_, exists := srv.listeners[hostname]
	return exists
}

func newTLSConfig(certs map[string]*backend.Certificate, defaultHost string) (*tls.Config, error) {
	config := &tls.Config{}

	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	pairs := make(map[string]tls.Certificate, len(certs))
	for h, c := range certs {
		cert, err := tls.X509KeyPair(c.PublicKey, c.PrivateKey)
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

func (s *srvPack) serve() {
	s.srv.Serve(s.netListener)
}
