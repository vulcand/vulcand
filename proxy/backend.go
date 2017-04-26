package proxy

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/vulcand/vulcand/engine"
)

type backend struct {
	mu          sync.Mutex
	id          string
	httpCfg     engine.HTTPBackendSettings
	httpTp      *http.Transport
	srvCfgsSeen bool
	srvCfgs     []engine.Server
}

func newBackend(beCfg engine.Backend, opts Options, beSrvCfgs []engine.Server) (*backend, error) {
	tpCfg, err := newTransportCfg(beCfg.HTTPSettings(), opts)
	if err != nil {
		return nil, errors.Wrap(err, "bad config")
	}
	return &backend{
		id:      beCfg.Id,
		httpCfg: beCfg.HTTPSettings(),
		httpTp:  newTransport(tpCfg),
		srvCfgs: beSrvCfgs,
	}, nil
}

func (be *backend) String() string {
	return fmt.Sprintf("backend(%v)", &be.id)
}

func (be *backend) close() error {
	be.httpTp.CloseIdleConnections()
	return nil
}

func (be *backend) update(beCfg engine.Backend, opts Options) (bool, error) {
	be.mu.Lock()
	defer be.mu.Unlock()

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

func (be *backend) upsertServer(beSrvCfg engine.Server) bool {
	be.mu.Lock()
	defer be.mu.Unlock()

	if i := be.indexOfServer(beSrvCfg.Id); i != -1 {
		if be.srvCfgs[i].URL == beSrvCfg.URL {
			return false
		}
		be.cloneSrvCfgsIfSeen()
		be.srvCfgs[i] = beSrvCfg
		return true
	}
	be.cloneSrvCfgsIfSeen()
	be.srvCfgs = append(be.srvCfgs, beSrvCfg)
	return true
}

func (be *backend) deleteServer(beSrvKey engine.ServerKey) bool {
	be.mu.Lock()
	defer be.mu.Unlock()

	i := be.indexOfServer(beSrvKey.Id)
	if i == -1 {
		log.Warnf("Cannot delete missing server %v from backend %v", beSrvKey.Id, be.id)
		return false
	}
	be.cloneSrvCfgsIfSeen()
	lastIdx := len(be.srvCfgs) - 1
	copy(be.srvCfgs[i:], be.srvCfgs[i+1:])
	be.srvCfgs[lastIdx] = engine.Server{}
	be.srvCfgs = be.srvCfgs[:lastIdx]
	return true
}

func (be *backend) snapshot() (*http.Transport, []engine.Server) {
	be.mu.Lock()
	defer be.mu.Unlock()

	be.srvCfgsSeen = true
	return be.httpTp, be.srvCfgs
}

func (be *backend) cloneSrvCfgsIfSeen() {
	if !be.srvCfgsSeen {
		return
	}
	size := len(be.srvCfgs)
	srvCfgs := make([]engine.Server, size, size*4/3)
	copy(srvCfgs, be.srvCfgs)
	be.srvCfgs = srvCfgs
	be.srvCfgsSeen = false
}

func (be *backend) indexOfServer(beSrvId string) int {
	for i := range be.srvCfgs {
		if be.srvCfgs[i].Id == beSrvId {
			return i
		}
	}
	return -1
}

func (be *backend) findServer(beSrvKey engine.ServerKey) (*engine.Server, bool) {
	i := be.indexOfServer(beSrvKey.Id)
	if i == -1 {
		return nil, false
	}
	return &be.srvCfgs[i], true
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

func newTransportCfg(httpCfg engine.HTTPBackendSettings, opts Options) (engine.TransportSettings, error) {
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
