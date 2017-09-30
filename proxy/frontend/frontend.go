package frontend

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/buffer"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/memmetrics"
	"github.com/vulcand/oxy/roundrobin"
	"github.com/vulcand/oxy/stream"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/plugin"
	"github.com/vulcand/vulcand/proxy"
	"github.com/vulcand/vulcand/proxy/backend"
	"github.com/vulcand/vulcand/proxy/rtmcollect"
)

// T represents a frontend instance. It implements http.Handler interface to be
// used with an http.Server. The implementation takes measures to collect round
// trip metrics for the frontend and all servers of an associated backend.
type T struct {
	mu         sync.Mutex
	ready      bool
	trustXFDH  bool
	cfg        engine.Frontend
	mwCfgs     map[engine.MiddlewareKey]engine.Middleware
	backend    *backend.T
	handler    http.Handler
	rtmCollect *rtmcollect.T
	listeners  plugin.FrontendListeners
}

// New returns a new frontend instance.
func New(cfg engine.Frontend, be *backend.T, opts proxy.Options,
	mwCfgs map[engine.MiddlewareKey]engine.Middleware,
	listeners plugin.FrontendListeners,
) *T {
	if mwCfgs == nil {
		mwCfgs = make(map[engine.MiddlewareKey]engine.Middleware)
	}
	fe := T{
		cfg:       cfg,
		trustXFDH: opts.TrustForwardHeader,
		mwCfgs:    mwCfgs,
		backend:   be,
		listeners: listeners,
	}
	return &fe
}

// Key returns the frontend storage key.
func (fe *T) Key() engine.FrontendKey {
	return fe.cfg.Key()
}

// BackendKey returns the storage key of an associated backend.
func (fe *T) BackendKey() engine.BackendKey {
	fe.mu.Lock()
	beKey := fe.backend.Key()
	fe.mu.Unlock()
	return beKey
}

// Route returns HTTP path. It should be used to configure an HTTP router to
// forward requests coming to the path to this frontend instance.
func (fe *T) Route() string {
	fe.mu.Lock()
	route := fe.cfg.Route
	fe.mu.Unlock()
	return route
}

// String returns a string representation of the instance to be used in logs.
func (fe *T) String() string {
	return fmt.Sprintf("frontend(%v)", fe.cfg.Id)
}

// Update updates the config and/or association with a backend.
func (fe *T) Update(feCfg engine.Frontend, be *backend.T) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if feCfg.Key() != fe.Key() {
		return errors.Errorf("invalid key, want=%v, got=%v", fe.Key(), feCfg.Key())
	}

	if !feCfg.HTTPSettings().Equals(fe.cfg.HTTPSettings()) {
		fe.cfg = feCfg
		fe.ready = false
	}
	if be != fe.backend {
		fe.backend = be
		fe.ready = false
	}
	return nil
}

// UpsertMiddleware upserts a middleware.
func (fe *T) UpsertMiddleware(mwCfg engine.Middleware) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	mwKey := engine.MiddlewareKey{FrontendKey: engine.FrontendKey{Id: fe.cfg.Id}, Id: mwCfg.Id}
	fe.mwCfgs[mwKey] = mwCfg
	fe.ready = false
}

// DeleteMiddleware deletes a middleware if there is one with the specified
// storage key or does nothing otherwise.
func (fe *T) DeleteMiddleware(mwKey engine.MiddlewareKey) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if _, ok := fe.mwCfgs[mwKey]; !ok {
		return
	}
	delete(fe.mwCfgs, mwKey)
	fe.ready = false
}

// OnBackendMutated should be called when state of the associated backend is
// changed, e.g. when a new backend server is added or something like that.
func (fe *T) OnBackendMutated() {
	fe.mu.Lock()
	fe.ready = false
	fe.mu.Unlock()
}

// CfgWithStats returns the frontend storage config with associated round trip
// stats.
func (fe *T) CfgWithStats() (engine.Frontend, bool, error) {
	fe.mu.Lock()
	rtmCollect := fe.rtmCollect
	feCfg := fe.cfg
	fe.mu.Unlock()

	if rtmCollect == nil {
		return engine.Frontend{}, false, nil
	}
	var err error
	if feCfg.Stats, err = rtmCollect.RTStats(); err != nil {
		return engine.Frontend{}, false, errors.Wrap(err, "failed to get stats")
	}
	return feCfg, true, nil
}

// AppendRTMTo appends frontend round-trip metrics to an aggregate.
func (fe *T) AppendRTMTo(aggregate *memmetrics.RTMetrics) {
	fe.mu.Lock()
	rtmCollect := fe.rtmCollect
	fe.mu.Unlock()

	if rtmCollect == nil {
		return
	}
	rtmCollect.AppendFeRTMTo(aggregate)
}

// AppendBeSrvRTMTo appends round-trip metrics of a backend server to aggregate.
// It does nothing if a server if the specified URL key does not exist.
func (fe *T) AppendBeSrvRTMTo(aggregate *memmetrics.RTMetrics, beSrvURLKey backend.SrvURLKey) {
	fe.mu.Lock()
	rtmCollect := fe.rtmCollect
	fe.mu.Unlock()

	if rtmCollect == nil {
		return
	}
	rtmCollect.AppendBeSrvRTMTo(aggregate, beSrvURLKey)
}

// AppendAllBeSrvRTMsTo appends round-trip metrics of all backend servers of
// the backend associated with the frontend to the respective aggregates. If an
// aggregate for a server is missing from the map then a new one is created.
func (fe *T) AppendAllBeSrvRTMsTo(aggregates map[backend.SrvURLKey]rtmcollect.BeSrvEntry) {
	fe.mu.Lock()
	rtmCollect := fe.rtmCollect
	fe.mu.Unlock()

	if rtmCollect == nil {
		return
	}
	rtmCollect.AppendAllBeSrvRTMsTo(aggregates)
}

// ServeHTTP implements http.Handler.
func (fe *T) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fe.getHandler().ServeHTTP(w, r)
}

func (fe *T) getHandler() http.Handler {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if !fe.ready {
		if err := fe.rebuild(); err != nil {
			log.Errorf("failed to rebuild frontend %v, err=%v", fe.cfg.Id, err)
			return proxy.DefaultNotFound
		}
		fe.ready = true
	}
	return fe.handler
}

func (fe *T) sortedMiddlewares() []engine.Middleware {
	vals := make([]engine.Middleware, 0, len(fe.mwCfgs))
	for _, m := range fe.mwCfgs {
		vals = append(vals, m)
	}
	sort.Sort(sort.Reverse(&middlewareSorter{ms: vals}))
	return vals
}

func (fe *T) rebuild() error {
	httpCfg := fe.cfg.HTTPSettings()
	httpTp, beSrvs := fe.backend.Snapshot()

	// set up forwarder
	fwd, err := forward.New(
		forward.RoundTripper(httpTp),
		forward.Rewriter(
			&forward.HeaderRewriter{
				Hostname:           httpCfg.Hostname,
				TrustForwardHeader: fe.trustXFDH || httpCfg.TrustForwardHeader,
			}),
		forward.PassHostHeader(httpCfg.PassHostHeader),
		forward.WebsocketTLSClientConfig(httpTp.TLSClientConfig),
		forward.Stream(httpCfg.Stream),
		forward.StreamingFlushInterval(time.Duration(httpCfg.StreamFlushIntervalNanoSecs)*time.Nanosecond),
		forward.StateListener(fe.listeners.ConnTck))

	// Add a round-trip metrics collector to the handlers chain.
	rc, err := rtmcollect.New(fwd)
	if err != nil {
		return errors.Wrap(err, "cannot create rtmCollect")
	}

	// Add a load balancer to the handlers chain.
	rr, err := roundrobin.New(rc, roundrobin.RoundRobinRequestRewriteListener(fe.listeners.RrRewriteListener))
	if err != nil {
		return errors.Wrap(err, "cannot create load balancer")
	}

	// Add a rebalancer to the handlers chain. It will readjust load balancer
	// weights based on error ratios.
	rb, err := roundrobin.NewRebalancer(rr, roundrobin.RebalancerRequestRewriteListener(fe.listeners.RbRewriteListener))
	if err != nil {
		return errors.Wrap(err, "cannot create rebalancer")
	}

	// create middlewares sorted by priority and chain them
	middlewares := fe.sortedMiddlewares()
	handlers := make([]http.Handler, len(middlewares))
	for i, mw := range middlewares {
		var prev http.Handler
		if i == 0 {
			prev = rb
		} else {
			prev = handlers[i-1]
		}
		h, err := mw.Middleware.NewHandler(prev)
		if err != nil {
			return errors.Wrapf(err, "cannot get middleware %v handler", mw.Id)
		}
		handlers[i] = h
	}

	var next http.Handler
	if len(handlers) != 0 {
		next = handlers[len(handlers)-1]
	} else {
		next = rb
	}

	// stream will retry and replay requests, fix encodings
	if httpCfg.FailoverPredicate == "" {
		httpCfg.FailoverPredicate = `IsNetworkError() && RequestMethod() == "GET" && Attempts() < 2`
	}

	var topHandler http.Handler
	if httpCfg.Stream {
		topHandler, err = stream.New(next)
	} else {
		topHandler, err = buffer.New(next,
			buffer.Retry(httpCfg.FailoverPredicate),
			buffer.MaxRequestBodyBytes(httpCfg.Limits.MaxBodyBytes),
			buffer.MemRequestBodyBytes(httpCfg.Limits.MaxMemBodyBytes))
	}
	if err != nil {
		return errors.Wrap(err, "failed to create handler")
	}

	syncServers(rb, beSrvs, rc)

	fe.handler = topHandler
	fe.rtmCollect = rc
	return nil
}

// syncServers syncs backend servers and rebalancer state.
func syncServers(balancer *roundrobin.Rebalancer, beSrvs []backend.Srv, watcher *rtmcollect.T) {
	// First, collect and parse servers to add
	newServers := make(map[backend.SrvURLKey]backend.Srv)
	for _, newBeSrv := range beSrvs {
		newServers[newBeSrv.URLKey()] = newBeSrv
	}

	// Memorize what endpoints exist in load balancer at the moment
	oldServers := make(map[backend.SrvURLKey]*url.URL)
	for _, oldBeSrvURL := range balancer.Servers() {
		oldServers[backend.NewSrvURLKey(oldBeSrvURL)] = oldBeSrvURL
	}

	// First, add endpoints, that should be added and are not in lb
	for newBeSrvURLKey, newBeSrv := range newServers {
		if _, ok := oldServers[newBeSrvURLKey]; !ok {
			if err := balancer.UpsertServer(newBeSrv.URL()); err != nil {
				log.Errorf("Failed to add %v, err: %s", newBeSrv.URL(), err)
			}
			watcher.UpsertServer(newBeSrv)
		}
	}

	// Second, remove endpoints that should not be there any more
	for oldBeSrvURLKey, oldBeSrvURL := range oldServers {
		if _, ok := newServers[oldBeSrvURLKey]; !ok {
			if err := balancer.RemoveServer(oldBeSrvURL); err != nil {
				log.Errorf("Failed to remove %v, err: %v", oldBeSrvURL, err)
			}
			watcher.RemoveServer(oldBeSrvURLKey)
		}
	}
}

type middlewareSorter struct {
	ms []engine.Middleware
}

func (s *middlewareSorter) Len() int {
	return len(s.ms)
}

func (s *middlewareSorter) Swap(i, j int) {
	s.ms[i], s.ms[j] = s.ms[j], s.ms[i]
}

func (s *middlewareSorter) Less(i, j int) bool {
	return s.ms[i].Priority < s.ms[j].Priority
}
