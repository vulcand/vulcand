package proxy

import (
	"fmt"
	"net/url"
	"sort"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/oxy/memmetrics"
	"github.com/mailgun/vulcand/engine"
)

func (mx *mux) frontendStats(key engine.FrontendKey) (*engine.RoundTripStats, error) {
	f, ok := mx.frontends[key]
	if !ok {
		return nil, fmt.Errorf("%v not found", key)
	}
	return f.watcher.rtStats()
}

func (mx *mux) backendStats(key engine.BackendKey) (*engine.RoundTripStats, error) {
	m, err := memmetrics.NewRTMetrics()
	if err != nil {
		return nil, err
	}
	for _, f := range mx.frontends {
		if f.backend.backend.Id != key.Id {
			continue
		}
		if err := f.watcher.collectMetrics(m); err != nil {
			return nil, err
		}
	}
	return engine.NewRoundTripStats(m)
}

func (mx *mux) serverStats(key engine.ServerKey) (*engine.RoundTripStats, error) {
	b, ok := mx.backends[key.BackendKey]
	if !ok {
		return nil, fmt.Errorf("%v not found", key.BackendKey)
	}
	srv, ok := b.findServer(key)
	if !ok {
		return nil, fmt.Errorf("%v not found", key)
	}

	u, err := url.Parse(srv.URL)
	if err != nil {
		return nil, err
	}

	m, err := memmetrics.NewRTMetrics()
	if err != nil {
		return nil, err
	}
	for _, f := range mx.frontends {
		if f.backend.backend.Id != key.BackendKey.Id {
			continue
		}
		if err := f.watcher.collectServerMetrics(m, u); err != nil {
			return nil, err
		}
	}
	return engine.NewRoundTripStats(m)
}

func (mx *mux) topFrontends(key *engine.BackendKey) ([]engine.Frontend, error) {
	frontends := []engine.Frontend{}
	for _, m := range mx.frontends {
		if key != nil && key.Id != m.backend.backend.Id {
			continue
		}
		f := m.frontend
		stats, err := m.watcher.rtStats()
		if err != nil {
			return nil, err
		}
		f.Stats = stats
		frontends = append(frontends, f)
	}
	sort.Stable(&frontendSorter{frontends: frontends})
	return frontends, nil
}

func (mx *mux) topServers(key *engine.BackendKey) ([]engine.Server, error) {
	servers := []engine.Server{}
	for _, f := range mx.frontends {
		if key != nil && key.Id != f.backend.backend.Id {
			continue
		}
		for _, s := range f.backend.servers {
			u, err := url.Parse(s.URL)
			if err != nil {
				return nil, err
			}
			stats, err := f.watcher.rtServerStats(u)
			if err != nil {
				return nil, err
			}
			s.Stats = stats
			servers = append(servers, s)
		}
	}
	sort.Stable(&serverSorter{es: servers})
	return servers, nil
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
