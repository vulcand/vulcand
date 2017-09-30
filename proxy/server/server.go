package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"

	proxyproto "github.com/armon/go-proxyproto"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/route"
	"github.com/vulcand/vulcand/conntracker"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/graceful"
	"github.com/vulcand/vulcand/proxy"
	"github.com/vulcand/vulcand/stapler"
	"golang.org/x/crypto/ocsp"
)

// T contains all that is necessary to run the HTTP(s) server. Note that it is
// not thread safe and therefore requires external synchronization.
type T struct {
	lsnCfg  engine.Listener
	router  http.Handler
	stapler stapler.Stapler
	connTck conntracker.ConnectionTracker
	serveWg *sync.WaitGroup

	srv          *graceful.Server
	scopedRouter http.Handler
	options      proxy.Options
	state        int
}

// New creates a new server instance.
func New(lsnCfg engine.Listener, router http.Handler, stapler stapler.Stapler,
	connTck conntracker.ConnectionTracker, wg *sync.WaitGroup,
) (*T, error) {
	scopedRouter, err := newScopeRouter(lsnCfg.Scope, router)
	if err != nil {
		return nil, err
	}
	return &T{
		lsnCfg:       lsnCfg,
		router:       router,
		stapler:      stapler,
		connTck:      connTck,
		serveWg:      wg,
		scopedRouter: scopedRouter,
		state:        srvStateInit,
	}, nil
}

func (s *T) Key() engine.ListenerKey {
	return s.lsnCfg.Key()
}

func (s *T) Address() engine.Address {
	return s.lsnCfg.Address
}

func (s *T) GetFile() (*proxy.FileDescriptor, error) {
	if !s.hasListeners() || s.srv == nil {
		return nil, nil
	}
	file, err := s.srv.GetFile()
	if err != nil {
		return nil, err
	}
	return &proxy.FileDescriptor{
		File:    file,
		Address: s.lsnCfg.Address,
	}, nil
}

func (s *T) String() string {
	return fmt.Sprintf("srv(%v, %v)", s.state, &s.lsnCfg)
}

// Start starts the server to handle a list of specified hosts.
func (s *T) Start(hostCfgs map[engine.HostKey]engine.Host) error {
	log.Infof("%s start", s)
	switch s.state {
	case srvStateInit:
		lsn, err := net.Listen(s.lsnCfg.Address.Network, s.lsnCfg.Address.Address)
		if err != nil {
			return err
		}

		lsn = &graceful.TCPKeepAliveListener{TCPListener: lsn.(*net.TCPListener)}

		if s.isProxyProto() {
			lsn = &proxyproto.Listener{
				Listener:           lsn,
				ProxyHeaderTimeout: s.options.ReadTimeout,
			}
		}

		if s.isTLS() {
			config, err := s.newTLSCfg(hostCfgs)
			if err != nil {
				return err
			}
			lsn = graceful.NewTLSListener(lsn, config)
		}
		s.srv = graceful.NewWithOptions(
			graceful.Options{
				Server:       s.newHTTPServer(),
				Listener:     lsn,
				StateHandler: s.connTck.RegisterStateChange,
			})
		s.state = srvStateActive
		s.serveWg.Add(1)
		go s.serve(s.srv)
		return nil
	case srvStateHijacked:
		s.state = srvStateActive
		s.serveWg.Add(1)
		go s.serve(s.srv)
		return nil
	}
	return errors.Errorf("%v Calling start in unsupported state", s)
}

// Shutdown gracefuly terminates the server if running.
func (s *T) Shutdown() {
	if s.srv != nil {
		s.srv.Close()
	}
}

func (s *T) Update(lsnCfg engine.Listener, hostCfgs map[engine.HostKey]engine.Host) error {
	// We can not listen for different protocols on the same socket
	if s.lsnCfg.Protocol != lsnCfg.Protocol {
		return errors.Errorf("conflicting protocol %s and %s", s.lsnCfg.Protocol, lsnCfg.Protocol)
	}
	if lsnCfg.Scope == s.lsnCfg.Scope && (&lsnCfg).SettingsEquals(&s.lsnCfg) {
		return nil
	}

	log.Infof("%v update %v", s, &lsnCfg)
	scopedRouter, err := newScopeRouter(lsnCfg.Scope, s.router)
	if err != nil {
		return errors.Wrap(err, "failed to create scoped handler")
	}
	s.scopedRouter = scopedRouter
	s.lsnCfg = lsnCfg
	if err := s.reloadTLSCfg(hostCfgs); err != nil {
		return errors.Wrap(err, "failed to reload TLS config")
	}
	return nil
}

func (s *T) TakeFile(fd *proxy.FileDescriptor, hostCfgs map[engine.HostKey]engine.Host) error {
	log.Infof("%s takeFile %v", s, fd)

	lsn, err := fd.ToListener()
	if err != nil {
		return errors.Wrapf(err, "failed to obtain listener for %v", fd)
	}

	tcpLsn, ok := lsn.(*net.TCPListener)
	if !ok {
		return errors.Errorf("bad listener type %T", lsn)
	}
	lsn = &graceful.TCPKeepAliveListener{TCPListener: tcpLsn}

	if s.isProxyProto() {
		lsn = &proxyproto.Listener{
			Listener:           lsn,
			ProxyHeaderTimeout: s.options.ReadTimeout,
		}
	}

	if s.isTLS() {
		config, err := s.newTLSCfg(hostCfgs)
		if err != nil {
			return errors.Wrap(err, "failed to create TLS config")
		}
		lsn = graceful.NewTLSListener(lsn, config)
	}

	s.srv = graceful.NewWithOptions(
		graceful.Options{
			Server:       s.newHTTPServer(),
			Listener:     lsn,
			StateHandler: s.connTck.RegisterStateChange,
		})
	s.state = srvStateHijacked
	return nil
}

// OnHostsUpdated is supposed to be called whenever a list of hosts is updated,
// or an OCSP notification for a host is received.
func (s *T) OnHostsUpdated(hostCfgs map[engine.HostKey]engine.Host) {
	if !s.isTLS() {
		return
	}
	if err := s.reloadTLSCfg(hostCfgs); err != nil {
		log.Errorf("Failed to reload TLS config: %v", err)
	}
}

func (s *T) reloadTLSCfg(hostCfgs map[engine.HostKey]engine.Host) error {
	if s.state != srvStateActive {
		return nil
	}

	gracefulServer, err := s.srv.HijackListener(
		s.newHTTPServer(),
		func(lsn net.Listener) (net.Listener, error) {
			lsn = &graceful.TCPKeepAliveListener{TCPListener: lsn.(*net.TCPListener)}

			if s.isProxyProto() {
				lsn = &proxyproto.Listener{
					Listener:           lsn,
					ProxyHeaderTimeout: s.options.ReadTimeout,
				}
			}

			if s.isTLS() {
				tlsCfg, err := s.newTLSCfg(hostCfgs)
				if err != nil {
					return nil, errors.Wrap(err, "failed to create TLS config")
				}
				lsn = graceful.NewTLSListener(lsn, tlsCfg)
			}
			return lsn, nil
		})
	if err != nil {
		return err
	}
	s.serveWg.Add(1)
	go s.serve(gracefulServer)

	s.srv.Close()
	s.srv = gracefulServer
	return nil
}

func (s *T) newHTTPServer() *http.Server {
	return &http.Server{
		Handler:        s.scopedRouter,
		ReadTimeout:    s.options.ReadTimeout,
		WriteTimeout:   s.options.WriteTimeout,
		MaxHeaderBytes: s.options.MaxHeaderBytes,
	}
}

func (s *T) isTLS() bool {
	return s.lsnCfg.Protocol == engine.HTTPS
}

func (s *T) isProxyProto() bool {
	return s.lsnCfg.ProxyProtocol == engine.PROXY_PROTO_V1
}

func (s *T) newTLSCfg(hostCfgs map[engine.HostKey]engine.Host) (*tls.Config, error) {
	config, err := s.lsnCfg.TLSConfig()
	if err != nil {
		return nil, err
	}

	if config.NextProtos == nil {
		// "h2" is required in order to enable HTTP 2: https://golang.org/src/net/http/server.go
		config.NextProtos = []string{"h2", "http/1.1"}
	}

	defaultHostName := ""
	pairs := map[string]tls.Certificate{}
	for _, hostCfg := range hostCfgs {
		if hostCfg.Settings.Default {
			defaultHostName = hostCfg.Name
		}
		c := hostCfg.Settings.KeyPair
		if c == nil {
			continue
		}
		keyPair, err := tls.X509KeyPair(c.Cert, c.Key)
		if err != nil {
			return nil, err
		}
		if hostCfg.Settings.OCSP.Enabled {
			log.Infof("%v OCSP is enabled for %v, resolvers: %v", s, hostCfg, hostCfg.Settings.OCSP.Responders)
			r, err := s.stapler.StapleHost(&hostCfg)
			if err != nil {
				log.Warningf("%v failed to staple %v, error %v", s, hostCfg, err)
			} else if r.Response.Status == ocsp.Good || r.Response.Status == ocsp.Revoked {
				keyPair.OCSPStaple = r.Staple
			} else {
				log.Warningf("%s got undefined status from OCSP responder: %v", s, r.Response.Status)
			}
		}
		pairs[hostCfg.Name] = keyPair
	}

	config.Certificates = make([]tls.Certificate, 0, len(pairs))
	if defaultHostName != "" {
		keyPair, ok := pairs[defaultHostName]
		if !ok {
			return nil, errors.Errorf("default host '%s' certificate is not passed", defaultHostName)
		}
		config.Certificates = append(config.Certificates, keyPair)
	}

	for hostName, keyPair := range pairs {
		if hostName != defaultHostName {
			config.Certificates = append(config.Certificates, keyPair)
		}
	}

	config.BuildNameToCertificate()
	return config, nil
}

func (s *T) hasListeners() bool {
	return s.state == srvStateActive || s.state == srvStateHijacked
}

func (s *T) serve(srv *graceful.Server) {
	defer s.serveWg.Done()
	log.Infof("%s serve", s)
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

func newScopeRouter(scope string, router http.Handler) (http.Handler, error) {
	if scope == "" {
		return router, nil
	}
	scopedRouter := route.NewMux()
	scopedRouter.SetNotFound(proxy.DefaultNotFound)
	if err := scopedRouter.Handle(scope, router); err != nil {
		return nil, err
	}
	return scopedRouter, nil
}
