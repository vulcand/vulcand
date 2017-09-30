package backend

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/proxy"
)

// T represents a backend type. It maintains a list of backend servers and
// returns them via Snapshot() functions.
type T struct {
	mu          sync.Mutex
	id          string
	httpCfg     engine.HTTPBackendSettings
	httpTp      *http.Transport
	srvCfgsSeen bool
	srvs        []Srv
}

// Srv represents a backend server instance.
type Srv struct {
	id        string
	rawURL    string
	parsedURL *url.URL
}

// Cfg returns engine.Server config of the backend server instance.
func (s *Srv) Cfg() engine.Server {
	return engine.Server{
		Id:  s.id,
		URL: s.rawURL,
	}
}

// SrvURLKey represents a server URL when it needs to be used as a map key.
type SrvURLKey struct {
	scheme string
	host   string
}

// NewSrvURLKey creates an SrvURLKey from a server URL.
func NewSrvURLKey(u *url.URL) SrvURLKey {
	return SrvURLKey{scheme: u.Scheme, host: u.Host}
}

// NewServer creates a backend server from a config fetched from a storage
// engine.
func NewServer(beSrvCfg engine.Server) (Srv, error) {
	parsedURL, err := url.Parse(beSrvCfg.URL)
	if err != nil {
		return Srv{}, errors.Wrapf(err, "bad url %v", beSrvCfg.URL)
	}
	return Srv{
		id:        beSrvCfg.Id,
		rawURL:    beSrvCfg.URL,
		parsedURL: parsedURL,
	}, nil
}

// URL return the backend server URL.
func (s *Srv) URL() *url.URL {
	return s.parsedURL
}

// URLKey returns the backend server SrvURLKey to be used as a key in maps.
func (s *Srv) URLKey() SrvURLKey {
	return NewSrvURLKey(s.parsedURL)
}

// New creates a new backend instance from a config fetched from a storage
// engine and proxy options. An initial list of backend servers can be provided.
func New(beCfg engine.Backend, opts proxy.Options, beSrvs []Srv) (*T, error) {
	tpCfg, err := newTransportCfg(beCfg.HTTPSettings(), opts)
	if err != nil {
		return nil, errors.Wrap(err, "bad config")
	}
	return &T{
		id:      beCfg.Id,
		httpCfg: beCfg.HTTPSettings(),
		httpTp:  newTransport(tpCfg),
		srvs:    beSrvs,
	}, nil
}

// Key returns storage backend key.
func (be *T) Key() engine.BackendKey {
	return engine.BackendKey{Id: be.id}
}

// String returns string backend representation to be used in logs.
func (be *T) String() string {
	return fmt.Sprintf("backend(%v)", &be.id)
}

// Close closes all idle connections to backends.
func (be *T) Close() error {
	// FIXME should not we close all connections here?
	be.httpTp.CloseIdleConnections()
	return nil
}

// Update updates the instance configuration.
func (be *T) Update(beCfg engine.Backend, opts proxy.Options) (bool, error) {
	be.mu.Lock()
	defer be.mu.Unlock()

	if beCfg.Key() != be.Key() {
		return false, errors.Errorf("invalid key, want=%v, got=%v", be.Key(), beCfg.Key())
	}

	// Config has not changed.
	if be.httpCfg.Equals(beCfg.HTTPSettings()) {
		return false, nil
	}

	tpCfg, err := newTransportCfg(beCfg.HTTPSettings(), opts)
	if err != nil {
		return false, errors.Wrap(err, "bad config")
	}

	// FIXME: But what about active connections?
	be.httpTp.CloseIdleConnections()

	be.httpCfg = beCfg.HTTPSettings()
	httpTp := newTransport(tpCfg)
	be.httpTp = httpTp
	return true, nil
}

// UpsertServer upserts a new backend server.
func (be *T) UpsertServer(beSrvCfg engine.Server) (bool, error) {
	be.mu.Lock()
	defer be.mu.Unlock()

	beSrv, err := NewServer(beSrvCfg)
	if err != nil {
		return false, errors.Wrapf(err, "bad config %v", beSrvCfg)
	}
	if i := be.indexOfServer(beSrvCfg.Id); i != -1 {
		if be.srvs[i].URLKey() == beSrv.URLKey() {
			return false, nil
		}
		be.cloneSrvCfgsIfSeen()
		be.srvs[i] = beSrv
		return true, nil
	}
	be.cloneSrvCfgsIfSeen()
	be.srvs = append(be.srvs, beSrv)
	return true, nil
}

// DeleteServer deletes a new backend server.
func (be *T) DeleteServer(beSrvKey engine.ServerKey) bool {
	be.mu.Lock()
	defer be.mu.Unlock()

	i := be.indexOfServer(beSrvKey.Id)
	if i == -1 {
		log.Warnf("Cannot delete missing server %v from backend %v", beSrvKey.Id, be.id)
		return false
	}
	be.cloneSrvCfgsIfSeen()
	lastIdx := len(be.srvs) - 1
	copy(be.srvs[i:], be.srvs[i+1:])
	be.srvs[lastIdx] = Srv{}
	be.srvs = be.srvs[:lastIdx]
	return true
}

// Snapshot returns configured HTTP transport instance and a list of backend
// servers. Due to copy-on-write semantic it is the returned server list is
// immutable from callers prospective and it is efficient to call this function
// as frequently as you want for it won't make excessive allocations.
func (be *T) Snapshot() (*http.Transport, []Srv) {
	be.mu.Lock()
	defer be.mu.Unlock()

	be.srvCfgsSeen = true
	return be.httpTp, be.srvs
}

// Server returns a backend server by a storage key if exists.
func (be *T) Server(beSrvKey engine.ServerKey) (Srv, bool) {
	be.mu.Lock()
	defer be.mu.Unlock()

	i := be.indexOfServer(beSrvKey.Id)
	if i == -1 {
		return Srv{}, false
	}
	return be.srvs[i], true
}

func (be *T) cloneSrvCfgsIfSeen() {
	if !be.srvCfgsSeen {
		return
	}
	size := len(be.srvs)
	srvCfgs := make([]Srv, size, size*4/3)
	copy(srvCfgs, be.srvs)
	be.srvs = srvCfgs
	be.srvCfgsSeen = false
}

func (be *T) indexOfServer(beSrvID string) int {
	for i := range be.srvs {
		if be.srvs[i].id == beSrvID {
			return i
		}
	}
	return -1
}

func newTransport(s engine.TransportSettings) *http.Transport {
	return &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   s.Timeouts.Dial,
			KeepAlive: s.KeepAlive.Period,
		}).Dial,
		ResponseHeaderTimeout: s.Timeouts.Read,
		TLSHandshakeTimeout:   s.Timeouts.TLSHandshake,
		MaxIdleConnsPerHost:   s.KeepAlive.MaxIdleConnsPerHost,
		TLSClientConfig:       s.TLS,
	}
}

func newTransportCfg(httpCfg engine.HTTPBackendSettings, opts proxy.Options) (engine.TransportSettings, error) {
	tpCfg, err := httpCfg.TransportSettings()
	if err != nil {
		return engine.TransportSettings{}, errors.Wrap(err, "invalid HTTP cfg")
	}
	// Apply global defaults if options are not set
	if tpCfg.Timeouts.Dial == 0 {
		tpCfg.Timeouts.Dial = opts.DialTimeout
	}
	if tpCfg.Timeouts.Read == 0 {
		tpCfg.Timeouts.Read = opts.ReadTimeout
	}
	return tpCfg, nil
}
