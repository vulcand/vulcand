package command

import (
	"fmt"
	"sort"

	"github.com/mailgun/vulcand/backend"
)

// High level overview of all locations sorted by activity (a.k.a top)
func hostsOverview(hosts []*backend.Host, limit int) Tree {
	r := &StringTree{
		Node: "[locations]",
	}

	if len(hosts) == 0 {
		return r
	}

	// Shuffle all locations
	locs := []*backend.Location{}
	for _, h := range hosts {
		for _, l := range h.Locations {
			locs = append(locs, l)
		}
	}

	// Sort locations by usage
	sort.Sort(&locSorter{locs: locs})

	count := 0
	for _, l := range locs {
		if limit > 0 && count >= limit {
			break
		}
		r.AddChild(locOverview(l))
		count += 1
	}

	return r
}

func hostOverview(h *backend.Host) *StringTree {
	r := &StringTree{
		Node: fmt.Sprintf("host[%s]", h.Name),
	}

	return r
}

func locOverview(l *backend.Location) *StringTree {
	s, f, periodSeconds := locStats(l)
	failRate := float64(0)
	if s+f != 0 {
		failRate = (float64(f) / float64(s+f)) * 100
	}
	r := &StringTree{
		Node: fmt.Sprintf("loc[%s, %s, %s, %0.1f requests/sec, %0.2f%%%% failures]",
			l.Id, l.Hostname, l.Path, float64(s+f)/float64(periodSeconds), failRate),
	}
	if failRate != 0 {
		r.Node = fmt.Sprintf("@r%s@w", r.Node)
	} else {
		r.Node = fmt.Sprintf("@g%s@w", r.Node)
	}
	return r
}

// Sorts locations by failures first, successes next
type locSorter struct {
	locs []*backend.Location
}

func (s *locSorter) Len() int {
	return len(s.locs)
}

func (s *locSorter) Swap(i, j int) {
	s.locs[i], s.locs[j] = s.locs[j], s.locs[i]
}

func (s *locSorter) Less(i, j int) bool {
	s1, f1, _ := locStats(s.locs[i])
	s2, f2, _ := locStats(s.locs[j])
	if f1 > f2 {
		return true
	}
	return s1 > s2
}

func locStats(loc *backend.Location) (successes, failures, periodSeconds int64) {
	for _, e := range loc.Upstream.Endpoints {
		if e.Stats != nil {
			successes += e.Stats.Successes
			failures += e.Stats.Failures
			periodSeconds = int64(e.Stats.PeriodSeconds)
		}
	}
	return successes, failures, periodSeconds
}
