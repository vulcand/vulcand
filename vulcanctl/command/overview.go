package command

import (
	"fmt"
	"sort"

	"github.com/mailgun/vulcand/backend"
)

// High level overview of hosts with basic stats
func hostsOverview(hosts []*backend.Host) Tree {
	r := &StringTree{
		Node: "[hosts]",
	}

	for _, h := range hosts {
		r.AddChild(hostOverview(h))
	}

	return r
}

func hostOverview(h *backend.Host) *StringTree {
	r := &StringTree{
		Node: fmt.Sprintf("host[%s]", h.Name),
	}

	if len(h.Locations) == 0 {
		return r
	}

	// Sort locations by usage
	sort.Sort(&locSorter{locs: h.Locations})

	for _, l := range h.Locations {
		r.AddChild(locOverview(l))
	}

	return r
}

func locOverview(l *backend.Location) *StringTree {
	s, f, periodSeconds := locStats(l)
	failRate := float64(0)
	if s+f != 0 {
		failRate = (float64(f) / float64(s+f)) * 100
	}
	return &StringTree{
		Node: fmt.Sprintf("loc[%s, %s, upstream=%s, %0.1f requests/sec, %0.2f%%%% failures]",
			l.Id, l.Path, l.Upstream.Id, float64(s+f)/float64(periodSeconds), failRate),
	}
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
	if f1 < f2 {
		return true
	}
	return s1 < s2
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
