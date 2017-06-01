package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/vulcand/oxy/buffer"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/roundrobin"
	"github.com/vulcand/oxy/stream"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/plugin"
)

var errorHandler = &DefaultNotFound{}

type frontend struct {
	mu        sync.Mutex
	ready     bool
	trustXFDH bool
	cfg       engine.Frontend
	mwCfgs    map[engine.MiddlewareKey]engine.Middleware
	backend   *backend
	handler   http.Handler
	watcher   *RTWatcher
	listeners plugin.FrontendListeners
}

func newFrontend(cfg engine.Frontend, be *backend, opts Options, mwCfgs map[engine.MiddlewareKey]engine.Middleware,
	listeners plugin.FrontendListeners,
) *frontend {
	if mwCfgs == nil {
		mwCfgs = make(map[engine.MiddlewareKey]engine.Middleware)
	}
	fe := frontend{
		cfg:       cfg,
		trustXFDH: opts.TrustForwardHeader,
		mwCfgs:    mwCfgs,
		backend:   be,
		listeners: listeners,
	}
	return &fe
}

func (fe *frontend) String() string {
	return fmt.Sprintf("frontend(%v)", fe.cfg.Id)
}

func (fe *frontend) update(feCfg engine.Frontend, be *backend) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if !feCfg.HTTPSettings().Equals(fe.cfg.HTTPSettings()) {
		fe.cfg = feCfg
		fe.ready = false
	}
	if be != fe.backend {
		fe.backend = be
		fe.ready = false
	}
}

func (fe *frontend) upsertMiddleware(mwCfg engine.Middleware) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	mwKey := engine.MiddlewareKey{FrontendKey: engine.FrontendKey{Id: fe.cfg.Id}, Id: mwCfg.Id}
	fe.mwCfgs[mwKey] = mwCfg
	fe.ready = false
}

func (fe *frontend) deleteMiddleware(mwKey engine.MiddlewareKey) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if _, ok := fe.mwCfgs[mwKey]; !ok {
		return
	}
	delete(fe.mwCfgs, mwKey)
	fe.ready = false
}

func (fe *frontend) onBackendMutated() {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	fe.ready = false
}

func (fe *frontend) getWatcher() *RTWatcher {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if fe.ready {
		return fe.watcher
	}
	return nil
}

func (fe *frontend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fe.getHandler().ServeHTTP(w, r)
}

func (fe *frontend) getHandler() http.Handler {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if !fe.ready {
		if err := fe.rebuild(); err != nil {
			log.Errorf("failed to rebuild frontend %v, err=%v", fe.cfg.Id, err)
			return errorHandler
		}
		fe.ready = true
	}
	return fe.handler
}

func (fe *frontend) sortedMiddlewares() []engine.Middleware {
	vals := make([]engine.Middleware, 0, len(fe.mwCfgs))
	for _, m := range fe.mwCfgs {
		vals = append(vals, m)
	}
	sort.Sort(sort.Reverse(&middlewareSorter{ms: vals}))
	return vals
}

func (fe *frontend) rebuild() error {
	httpCfg := fe.cfg.HTTPSettings()
	httpTp, beSrvCfgs := fe.backend.snapshot()

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

	// rtwatcher will be observing and aggregating metrics
	watcher, err := NewWatcher(fwd)
	if err != nil {
		return errors.Wrap(err, "cannot create watcher")
	}

	// Create a load balancer
	rr, err := roundrobin.New(watcher, roundrobin.RoundRobinRequestRewriteListener(fe.listeners.RrRewriteListener))
	if err != nil {
		return errors.Wrap(err, "cannot create load balancer")
	}

	// Rebalancer will readjust load balancer weights based on error ratios
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

	syncServers(rb, beSrvCfgs, watcher)

	fe.handler = topHandler
	fe.watcher = watcher
	return nil
}

// syncs backend servers and rebalancer state
func syncServers(balancer *roundrobin.Rebalancer, beSrvCfgs []engine.Server, watcher *RTWatcher) {
	// First, collect and parse servers to add
	newServers := map[string]*url.URL{}
	for _, beSrvCfg := range beSrvCfgs {
		u, err := url.Parse(beSrvCfg.URL)
		if err != nil {
			log.Errorf("failed to parse url %v", beSrvCfg.URL)
			continue
		}
		newServers[beSrvCfg.URL] = u
	}

	// Memorize what endpoints exist in load balancer at the moment
	existingServers := map[string]*url.URL{}
	for _, s := range balancer.Servers() {
		existingServers[s.String()] = s
	}

	// First, add endpoints, that should be added and are not in lb
	for _, s := range newServers {
		if _, exists := existingServers[s.String()]; !exists {
			if err := balancer.UpsertServer(s); err != nil {
				log.Errorf("failed to add %v, err: %s", s, err)
			}
			watcher.upsertServer(s)
		}
	}

	// Second, remove endpoints that should not be there any more
	for k, v := range existingServers {
		if _, exists := newServers[k]; !exists {
			if err := balancer.RemoveServer(v); err != nil {
				log.Errorf("failed to remove %v, err: %v", v, err)
			} else {
				log.Infof("removed %v", v)
			}
			watcher.removeServer(v)
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
