package command

import (
	"fmt"
	"io"
	"sort"

	"github.com/buger/goterm"
	"github.com/mailgun/vulcand/backend"
)

// High level overview of all locations sorted by activity (a.k.a top)
func hostsOverview(hosts []*backend.Host, limit int) string {

	t := goterm.NewTable(0, 10, 5, ' ', 0)
	fmt.Fprintf(t, "Id\tHostname\tPath\tRequests/sec\tFail rate\n")

	if len(hosts) == 0 {
		return t.String()
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
		locOverview(t, l)
		count += 1
	}

	return t.String()
}

func locOverview(w io.Writer, l *backend.Location) {
	s := locStats(l)
	failRate := float64(0)
	if s.Successes+s.Failures != 0 {
		failRate = (float64(s.Failures) / float64(s.Failures+s.Successes)) * 100
	}
	reqsSec := float64(s.Successes+s.Failures) / float64(s.PeriodSeconds)

	failRateS := fmt.Sprintf("%0.2f", failRate)
	if failRate != 0 {
		failRateS = goterm.Color(failRateS, goterm.RED)
	} else {
		failRateS = goterm.Color(failRateS, goterm.GREEN)
	}

	fmt.Fprintf(w, "%s\t%s\t%s\t%0.1f\t%s\n",
		l.Id, l.Hostname, l.Path, reqsSec, failRateS)

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
	s1 := locStats(s.locs[i])
	s2 := locStats(s.locs[j])

	return s1.Failures > s2.Failures || s1.Successes > s2.Successes
}

func locStats(loc *backend.Location) *backend.EndpointStats {
	stats := &backend.EndpointStats{}
	for _, e := range loc.Upstream.Endpoints {
		if e.Stats != nil {
			stats.Successes += e.Stats.Successes
			stats.Failures += e.Stats.Failures
			stats.PeriodSeconds = e.Stats.PeriodSeconds
		}
	}
	return stats
}
