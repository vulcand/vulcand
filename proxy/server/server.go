package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
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
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/crypto/ocsp"
)

type getCertificateFunc func(*tls.ClientHelloInfo) (*tls.Certificate, error)

// T contains all that is necessary to run the HTTP(s) server. Note that it is
// not thread safe and therefore requires external synchronization.
type T struct {
	lsnCfg  engine.Listener
	router  http.Handler
	stapler stapler.Stapler
	connTck conntracker.ConnectionTracker
	serveWg *sync.WaitGroup

	autoCertCache autocert.Cache

	srv          *graceful.Server
	scopedRouter http.Handler
	options      proxy.Options
	state        int
}

// New creates a new server instance.
func New(lsnCfg engine.Listener, router http.Handler, stapler stapler.Stapler,
	connTck conntracker.ConnectionTracker, autoCertCache autocert.Cache, wg *sync.WaitGroup,
) (*T, error) {
	scopedRouter, err := newScopeRouter(lsnCfg.Scope, router)
	if err != nil {
		return nil, err
	}
	return &T{
		lsnCfg: lsnCfg,
		router: router,

		stapler:       stapler,
		connTck:       connTck,
		autoCertCache: autoCertCache,
		serveWg:       wg,
		scopedRouter:  scopedRouter,
		state:         srvStateInit,
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
	getCertFuncs := map[string]getCertificateFunc{}

	for _, hostCfg := range hostCfgs {
		// Capture default hostname for later use
		if hostCfg.Settings.Default {
			defaultHostName = hostCfg.Name
		}

		// If KeyPair-based cert
		if hostCfg.Settings.KeyPair != nil {

			//If autocert is also set, log a warning but proceed with non-autocert
			if hostCfg.Settings.AutoCert != nil {
				log.Warningf("Host %v has a KeyPair and AutoCert enabled. KeyPair takes precedence. Autocert generation disabled.", hostCfg.Name)
			}

			// Get the certificate for this host out of settings and remember it
			cert, err := certForHost(hostCfg)
			if err != nil {
				log.Errorf("Unable to get certificate from host %s due to error: %v.", hostCfg.Name, err)
				continue
			}

			//Staple the OCSP response to the cert
			ocspStapleToCert(s.stapler, hostCfg, &cert)

			pairs[hostCfg.Name] = cert

		} else if hostCfg.Settings.AutoCert != nil {

			// Generate a certificate-getting function for this host which will be called later as needed.
			// This function will staple OCSP response when cert is generated.
			getCertFunc, err := certFuncForHost(hostCfg, s.autoCertCache, s.stapler)
			if err != nil {
				log.Errorf("Unable to generate GetCertificate function for host %s due to error: %v.", hostCfg.Name, err)
				continue
			}
			getCertFuncs[hostCfg.Name] = getCertFunc
		}
	}

	// Convert the hostname->cert mappings into an array with defaultHostName's cert coming first
	config.Certificates, err = tlsCertArray(pairs, defaultHostName)
	if err != nil {
		return nil, err
	}

	config.BuildNameToCertificate()

	// Generate an aggergate GetCertificate that calls individual host's GetCertificate generated above.
	config.GetCertificate = getCertFuncAggregate(getCertFuncs)

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

// Returns a GetCertificate function for this host based on AutoCert settings,
// by generating an autocert manager for the host.
// It optionally will staple the OCSP response to the cert when it is generated,
// if OCSP stapling is enabled for the host.
func certFuncForHost(hostCfg engine.Host, autoCertCache autocert.Cache, s stapler.Stapler) (getCertificateFunc, error) {
	ac := hostCfg.Settings.AutoCert

	// Each host gets its own Autocert Manager - this allows individual
	// certs to use different autocert authorities, as well as auth keys
	autoCertMgr := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autoCertCache,
		HostPolicy: autocert.HostWhitelist(hostCfg.Name),
		Email:      ac.Email,
	}

	if ac.RenewBefore > 0 {
		autoCertMgr.RenewBefore = ac.RenewBefore
	}

	// if either directory or key are non-empty, we need to generate
	// a custom ACME client to override either.
	if ac.Key != "" || ac.DirectoryURL != "" {

		// If DirectoryURL is empty, the default Let's Encrypt URL will be picked.
		autoCertMgr.Client = &acme.Client{
			DirectoryURL: ac.DirectoryURL,
		}

		// If Key is non-empty, then decode it as RSA or EC which are the only two keys
		// we support. Go's crypto library doesn't support a generic function to provide back
		// a private key interface.
		if ac.Key != "" {
			block, _ := pem.Decode([]byte(ac.Key))
			if block == nil {
				return nil, fmt.Errorf("Autocert Key PEM Block for Host %s is invalid.", hostCfg.Name)
			} else if block.Type == "RSA PRIVATE KEY" {
				rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err != nil {
					return nil, errors.Wrapf(err, "Error parsing Autocert Key block of type %s, for Host %s, as an RSA Private Key.", block.Type, hostCfg.Name)
				}
				autoCertMgr.Client.Key = rsaPrivateKey
			} else if block.Type == "EC PRIVATE KEY" {
				ecPrivateKey, err := x509.ParseECPrivateKey(block.Bytes)
				if err != nil {
					return nil, errors.Wrapf(err, "Error parsing Autocert Key block of type %s, for Host %s, as an ECDSA Private Key.", block.Type, hostCfg.Name)
				}
				autoCertMgr.Client.Key = ecPrivateKey
			} else {
				return nil, fmt.Errorf("AutoCert Private Key for Host %s is of unrecognized type: %s. Supported types"+
					"are RSA PRIVATE KEY and EC PRIVATE KEY.", hostCfg.Name, block.Type)
			}
		}
	}

	getCertFuncForStapling := stapler.WithGetCertFunc(hostCfg.Name, stapler.GetCertificateFunc(autoCertMgr.GetCertificate))

	// Wrap the GetCert for this host, so we can generate and staple
	// an optional OCSP response when requested.
	stapledGetCert := func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		keyPair, err := autoCertMgr.GetCertificate(info)
		ocspStapleToCert(s, hostCfg, keyPair, getCertFuncForStapling)
		return keyPair, err
	}

	return stapledGetCert, nil
}

// Generate an aggregate GetCertificate function over GetCertificate functions for
// each host that we generated using the certFuncForHost function above.
// This allows all of those functions to masquerade as one uber function that can
// get a certificate (through autocert) for any host.
func getCertFuncAggregate(getCertFuncs map[string]getCertificateFunc) getCertificateFunc {
	return func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if getCertificateFunc, ok := getCertFuncs[info.ServerName]; ok {
			// We have a get certificate function for this host - allow AutoCertManager to
			// provide this one, in case there's expiry/renewal to be done.
			cert, err := getCertificateFunc(info)
			if err != nil {
				log.Errorf("Failed to generate Autocert for ServerName: %s. Error: %v.", info.ServerName, err)
			}
			return cert, err
		} else {
			return nil, nil
		}
	}
}

func tlsCertArray(pairs map[string]tls.Certificate, defaultHostName string) ([]tls.Certificate, error) {
	arr := make([]tls.Certificate, 0, len(pairs))
	if defaultHostName != "" {
		keyPair, ok := pairs[defaultHostName]
		if !ok {
			return nil, errors.Errorf("default host '%s' certificate is not passed", defaultHostName)
		}
		arr = append(arr, keyPair)
	}

	for hostName, keyPair := range pairs {
		if hostName != defaultHostName {
			arr = append(arr, keyPair)
		}
	}
	return arr, nil
}

func ocspStapleToCert(stapler stapler.Stapler, hostCfg engine.Host, keyPair *tls.Certificate, opts ...stapler.StapleHostOption) {
	if !hostCfg.Settings.OCSP.Enabled {
		return
	}

	log.Infof("OCSP is enabled for %v, resolvers: %v", hostCfg, hostCfg.Settings.OCSP.Responders)

	r, err := stapler.StapleHost(&hostCfg, opts...)

	if err != nil {
		log.Warningf("Failed to staple %v, error %v", hostCfg, err)
		return
	}

	if r.Response.Status != ocsp.Good && r.Response.Status != ocsp.Revoked {
		log.Warningf("Got undefined status from OCSP responder: %v", r.Response.Status)
		return
	}

	keyPair.OCSPStaple = r.Staple
}

// Returns a certificate based on a hosts KeyPair settings.
func certForHost(hostCfg engine.Host) (tls.Certificate, error) {
	c := hostCfg.Settings.KeyPair
	cert, err := tls.X509KeyPair(c.Cert, c.Key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return cert, nil
}
