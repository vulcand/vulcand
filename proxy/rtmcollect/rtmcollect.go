package rtmcollect

import (
	"net/http"
	"sync"

	"github.com/mailgun/timetools"
	"github.com/pkg/errors"
	"github.com/vulcand/oxy/memmetrics"
	"github.com/vulcand/oxy/utils"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/proxy/backend"
)

// NewRTMetrics is a convenience wrapper around memmetrics.NewRTMetrics() to
// get rid of unnecessary error check that never actually happens.
func NewRTMetrics() *memmetrics.RTMetrics {
	rtm, err := memmetrics.NewRTMetrics()
	if err != nil {
		panic(errors.Wrap(err, "must never fail"))
	}
	return rtm
}

// T watches and aggregates round-trip metrics for calls to its ServeHTTP
// function and all backend servers that handle it.
type T struct {
	mu        sync.Mutex
	rtm       *memmetrics.RTMetrics
	beSrvRTMs map[backend.SrvURLKey]BeSrvEntry
	clock     timetools.TimeProvider
	handler   http.Handler
}

// BeSrvEntry used to store a backend server storage config along with
// respective round-trip metrics in a map.
type BeSrvEntry struct {
	beSrvCfg engine.Server
	rtm      *memmetrics.RTMetrics
}

// CfgWithStats returns a backend server storage config along with round-trip
// stats.
func (e *BeSrvEntry) CfgWithStats() engine.Server {
	cfg := e.beSrvCfg
	var err error
	cfg.Stats, err = engine.NewRoundTripStats(e.rtm)
	if err != nil {
		panic(errors.Wrap(err, "must never fail"))
	}
	return cfg
}

// New returns a new round-trip metrics collector instance.
func New(handler http.Handler) (*T, error) {
	feRTM, err := memmetrics.NewRTMetrics()
	if err != nil {
		return nil, err
	}
	return &T{
		rtm:       feRTM,
		beSrvRTMs: make(map[backend.SrvURLKey]BeSrvEntry),
		clock:     &timetools.RealTime{},
		handler:   handler,
	}, nil
}

// ServeHTTP implements http.Handler.
func (c *T) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := c.clock.UtcNow()
	pw := &utils.ProxyWriter{W: w}
	c.handler.ServeHTTP(pw, req)
	diff := c.clock.UtcNow().Sub(start)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.rtm.Record(pw.Code, diff)
	if beSrvEnt, ok := c.beSrvRTMs[backend.NewSrvURLKey(req.URL)]; ok {
		beSrvEnt.rtm.Record(pw.Code, diff)
	}
}

// RTStats returns round-trip stats of the associated frontend.
func (c *T) RTStats() (*engine.RoundTripStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return engine.NewRoundTripStats(c.rtm)
}

// UpsertServer upserts a backend server to collect round-trip metrics for.
func (c *T) UpsertServer(beSrv backend.Srv) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.beSrvRTMs[beSrv.URLKey()] = BeSrvEntry{beSrv.Cfg(), NewRTMetrics()}
}

// RemoveServer removes a backend server from the list of servers that it
// collects round-trip metrics for.
func (c *T) RemoveServer(beSrvURLKey backend.SrvURLKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.beSrvRTMs, beSrvURLKey)
}

// AppendFeRTMTo appends frontend round-trip metrics to an aggregate.
func (c *T) AppendFeRTMTo(aggregate *memmetrics.RTMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return aggregate.Append(c.rtm)
}

// AppendBeSrvRTMTo appends round-trip metrics of a backend server to aggregate.
// It does nothing if a server if the specified URL key does not exist.
func (c *T) AppendBeSrvRTMTo(aggregate *memmetrics.RTMetrics, beSrvURLKey backend.SrvURLKey) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if beSrvEnt, ok := c.beSrvRTMs[beSrvURLKey]; ok {
		return aggregate.Append(beSrvEnt.rtm)
	}
	return nil
}

// AppendAllBeSrvRTMsTo appends round-trip metrics of all backend servers of
// the backend associated with the frontend to the respective aggregates. If an
// aggregate for a server is missing from the map then a new one is created.
func (c *T) AppendAllBeSrvRTMsTo(aggregates map[backend.SrvURLKey]BeSrvEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for beSrvURLKey, beSrvEnt := range c.beSrvRTMs {
		aggregate, ok := aggregates[beSrvURLKey]
		if !ok {
			aggregate = BeSrvEntry{beSrvEnt.beSrvCfg, NewRTMetrics()}
			aggregates[beSrvURLKey] = aggregate
		}
		aggregate.rtm.Append(beSrvEnt.rtm)
	}
}
