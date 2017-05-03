package proxy

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/vulcand/oxy/memmetrics"
	"github.com/vulcand/vulcand/engine"
)

func (m *mux) emitMetrics() error {
	c := m.options.MetricsClient

	// Emit connection stats
	counts := m.incomingConnTracker.Counts()
	for state, values := range counts {
		for addr, count := range values {
			c.Gauge(c.Metric("conns", addr, state.String()), count, 1)
		}
	}

	// Emit frontend metrics stats
	frontends, err := m.TopFrontends(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get top frontends")
	}
	for _, fe := range frontends {
		fem := c.Metric("frontend", strings.Replace(fe.Id, ".", "_", -1))
		s := fe.Stats
		for _, scode := range s.Counters.StatusCodes {
			// response codes counters
			c.Gauge(fem.Metric("code", strconv.Itoa(scode.Code)), scode.Count, 1)
		}
		// network errors
		c.Gauge(fem.Metric("neterr"), s.Counters.NetErrors, 1)
		// requests
		c.Gauge(fem.Metric("reqs"), s.Counters.Total, 1)

		// round trip times in microsecond resolution
		for _, b := range s.LatencyBrackets {
			c.Gauge(fem.Metric("rtt", strconv.Itoa(int(b.Quantile*10.0))), int64(b.Value/time.Microsecond), 1)
		}
	}
	return nil
}

func (m *mux) FrontendStats(feKey engine.FrontendKey) (*engine.RoundTripStats, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	fe, ok := m.frontends[feKey]
	if !ok {
		return nil, errors.Errorf("%v not found", feKey.Id)
	}
	watcher := fe.getWatcher()
	if watcher == nil {
		return nil, errors.Errorf("%v not used", feKey.Id)
	}
	return watcher.rtStats()
}

func (m *mux) BackendStats(beKey engine.BackendKey) (*engine.RoundTripStats, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	rtm, err := memmetrics.NewRTMetrics()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create RTM")
	}
	for _, fe := range m.frontends {
		if fe.backend.id != beKey.Id {
			continue
		}
		watcher := fe.getWatcher()
		if watcher == nil {
			continue
		}
		if err := watcher.collectMetrics(rtm); err != nil {
			return nil, errors.Wrapf(err, "failed to collect metrics for %v", fe.cfg.Id)
		}
	}
	return engine.NewRoundTripStats(rtm)
}

func (m *mux) ServerStats(beSrvKey engine.ServerKey) (*engine.RoundTripStats, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	beEnt, ok := m.backends[beSrvKey.BackendKey]
	if !ok {
		return nil, errors.Errorf("%v not found", beSrvKey.BackendKey)
	}
	beSrvCfg, ok := beEnt.backend.findServer(beSrvKey)
	if !ok {
		return nil, errors.Errorf("%v not found", beSrvKey)
	}

	u, err := url.Parse(beSrvCfg.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "bad backend server url %v", beSrvCfg)
	}

	rtm, err := memmetrics.NewRTMetrics()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create RTM")
	}
	for _, fe := range m.frontends {
		if fe.backend.id != beSrvKey.BackendKey.Id {
			continue
		}
		watcher := fe.getWatcher()
		if watcher == nil {
			continue
		}
		if err := watcher.collectServerMetrics(rtm, u); err != nil {
			return nil, errors.Wrapf(err, "failed to collect metrics for %v", fe.cfg.Id)
		}
	}
	return engine.NewRoundTripStats(rtm)
}

// TopFrontends returns locations sorted by criteria (faulty, slow, most used)
// if hostname or backendId is present, will filter out locations for that host or backendId
func (m *mux) TopFrontends(beKey *engine.BackendKey) ([]engine.Frontend, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	feCfgs := []engine.Frontend{}
	for _, fe := range m.frontends {
		if beKey != nil && beKey.Id != fe.backend.id {
			continue
		}
		watcher := fe.getWatcher()
		if watcher == nil {
			continue
		}
		feCfg := fe.cfg
		stats, err := watcher.rtStats()
		if err != nil {
			return nil, errors.Wrapf(err, "cannot get stats from %v", fe.cfg.Id)
		}
		feCfg.Stats = stats
		feCfgs = append(feCfgs, feCfg)
	}
	sort.Stable(&frontendSorter{frontends: feCfgs})
	return feCfgs, nil
}

// TopServers returns endpoints sorted by criteria (faulty, slow, most used)
// if backendId is not empty, will filter out endpoints for that backendId
func (m *mux) TopServers(beKey *engine.BackendKey) ([]engine.Server, error) {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	metrics := map[string]*sval{}
	for _, fe := range m.frontends {
		watcher := fe.getWatcher()
		if watcher == nil {
			continue
		}
		if beKey != nil && beKey.Id != fe.backend.id {
			continue
		}
		for _, beSrvCfg := range fe.backend.srvCfgs {
			val, ok := metrics[beSrvCfg.URL]
			if !ok {
				sval, err := newSval(beSrvCfg)
				if err != nil {
					return nil, errors.Wrapf(err, "bad backend server %v", beSrvCfg)
				}
				metrics[beSrvCfg.URL] = sval
				val = sval
			}
			if err := watcher.collectServerMetrics(val.rtm, val.url); err != nil {
				return nil, errors.Wrapf(err, "failed to collect server metrics from %v", fe.cfg.Id)
			}
		}
	}
	beSrvCfgs := make([]engine.Server, 0, len(metrics))
	for _, v := range metrics {
		stats, err := engine.NewRoundTripStats(v.rtm)
		if err != nil {
			return nil, errors.Wrap(err, "cannot create RTS")
		}
		v.srv.Stats = stats
		beSrvCfgs = append(beSrvCfgs, *v.srv)
	}
	sort.Stable(&serverSorter{es: beSrvCfgs})
	return beSrvCfgs, nil
}

type sval struct {
	url *url.URL
	srv *engine.Server
	rtm *memmetrics.RTMetrics
}

func newSval(beSrvCfg engine.Server) (*sval, error) {
	rtm, err := memmetrics.NewRTMetrics()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create RTM")
	}
	u, err := url.Parse(beSrvCfg.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "bad url %v", beSrvCfg.URL)
	}
	return &sval{srv: &beSrvCfg, rtm: rtm, url: u}, nil
}

type frontendSorter struct {
	frontends []engine.Frontend
}

func (s *frontendSorter) Len() int {
	return len(s.frontends)
}

func (s *frontendSorter) Swap(i, j int) {
	s.frontends[i], s.frontends[j] = s.frontends[j], s.frontends[i]
}

func (s *frontendSorter) Less(i, j int) bool {
	return cmpStats(s.frontends[i].Stats, s.frontends[j].Stats)
}

type serverSorter struct {
	es []engine.Server
}

func (s *serverSorter) Len() int {
	return len(s.es)
}

func (s *serverSorter) Swap(i, j int) {
	s.es[i], s.es[j] = s.es[j], s.es[i]
}

func (s *serverSorter) Less(i, j int) bool {
	return cmpStats(s.es[i].Stats, s.es[j].Stats)
}

func cmpStats(s1, s2 *engine.RoundTripStats) bool {
	// Items that have network errors go first
	if s1.NetErrorRatio() != 0 || s2.NetErrorRatio() != 0 {
		return s1.NetErrorRatio() > s2.NetErrorRatio()
	}

	// Items that have application level errors go next
	if s1.AppErrorRatio() != 0 || s2.AppErrorRatio() != 0 {
		return s1.AppErrorRatio() > s2.AppErrorRatio()
	}

	// More highly loaded items go next
	return s1.Counters.Total > s2.Counters.Total
}
