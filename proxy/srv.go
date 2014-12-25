package proxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/mailgun/vulcand/engine"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/manners"
)

// srv contains all that is necessary to run the HTTP(s) server. server does not work on its own,
// it heavily depends on MuxServer and acts as its internal data structure.
type srv struct {
	defaultHost string
	mux         *mux
	srv         *manners.GracefulServer
	proxy       http.Handler
	listener    engine.Listener
	keyPairs    map[engine.HostKey]engine.KeyPair
	options     Options
	state       int
}

func (s *srv) GetFile() (*FileDescriptor, error) {
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

func (s *srv) String() string {
	return fmt.Sprintf("%s->srv(%v, %v)", s.mux, s.state, &s.listener)
}

func newSrv(m *mux, l engine.Listener) (*srv, error) {
	defaultHost := ""
	keyPairs := make(map[engine.HostKey]engine.KeyPair)
	for hk, h := range m.hosts {
		if h.Settings.KeyPair != nil {
			keyPairs[hk] = *h.Settings.KeyPair
		}
		if h.Settings.Default {
			defaultHost = hk.Name
		}
	}

	return &srv{
		mux:         m,
		proxy:       m.router,
		listener:    l,
		defaultHost: defaultHost,
		keyPairs:    keyPairs,
		state:       srvStateInit,
	}, nil
}

func (s *srv) deleteKeyPair(hk engine.HostKey) error {
	delete(s.keyPairs, hk)
	return s.reload()
}

func (s *srv) isTLS() bool {
	return s.listener.Protocol == engine.HTTPS
}

func (s *srv) upsertKeyPair(hk engine.HostKey, keyPair *engine.KeyPair) error {
	old, exists := s.keyPairs[hk]
	if exists && old.Equals(keyPair) {
		return nil
	}
	s.keyPairs[hk] = *keyPair
	return s.reload()
}

func (s *srv) setDefaultHost(host engine.Host) error {
	oldDefault := s.defaultHost
	if host.Settings.Default {
		s.defaultHost = host.Name
	}
	if oldDefault != s.defaultHost && s.isTLS() {
		return s.reload()
	}
	return nil
}

func (s *srv) isServing() bool {
	return s.state == srvStateActive
}

func (s *srv) hasListeners() bool {
	return s.state == srvStateActive || s.state == srvStateHijacked
}

func (s *srv) takeFile(f *FileDescriptor) error {
	log.Infof("%s takeFile %v", s, f)

	listener, err := f.ToListener()
	if err != nil {
		return err
	}

	if s.isTLS() {
		tcpListener, ok := listener.(*net.TCPListener)
		if !ok {
			return fmt.Errorf(`%s failed to take file descriptor - it is running in TLS mode so I need a TCP listener, 
but the file descriptor that was given corresponded to a listener of type %T. More about file descriptor: %s`, listener, s, f)
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

func (s *srv) newHTTPServer() *http.Server {
	return &http.Server{
		Handler:        s.proxy,
		ReadTimeout:    s.options.ReadTimeout,
		WriteTimeout:   s.options.WriteTimeout,
		MaxHeaderBytes: s.options.MaxHeaderBytes,
	}
}

func (s *srv) reload() error {
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

func (s *srv) shutdown() {
	if s.srv != nil {
		s.srv.Close()
	}
}

func newTLSConfig(keyPairs map[engine.HostKey]engine.KeyPair, defaultHost string) (*tls.Config, error) {
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
		pairs[h.Name] = keyPair
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

func (s *srv) start() error {
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
	return fmt.Errorf("%v Calling start in unsupported state", s)
}

func (s *srv) serve(srv *manners.GracefulServer) {
	log.Infof("%s serve", s)

	s.mux.wg.Add(1)
	defer s.mux.wg.Done()

	srv.ListenAndServe()

	log.Infof("%v stop", s)
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
