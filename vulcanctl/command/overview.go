package command

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/mailgun/vulcand/backend"
)

// High level overview of all locations sorted by activity (a.k.a top)
func hostsOverview(hosts []*backend.Host, limit int) string {

	t := goterm.NewTable(0, 10, 5, ' ', 0)
	fmt.Fprint(t, "Id\tHostname\tPath\tReqs/sec\t95ile [ms]\t99ile [ms]\tStatus codes %%%%\tNetwork errors %%%%\n")

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

func upstreamsOverview(upstreams []*backend.Upstream, limit int) string {

	t := goterm.NewTable(0, 10, 5, ' ', 0)
	fmt.Fprint(t, "UpstreamId\tId\tUrl\tReqs/sec\t95ile [ms]\t99ile [ms]\tStatus codes %%%%\tNetwork errors %%%%\n")

	// Sort endpoints
	endpoints := []*backend.Endpoint{}
	for _, u := range upstreams {
		for _, e := range u.Endpoints {
			endpoints = append(endpoints, e)
		}
	}

	sort.Sort(&endpointSorter{es: endpoints})

	count := 0
	for _, e := range endpoints {
		if limit > 0 && count >= limit {
			break
		}
		endpointOverview(t, e)
		count += 1
	}

	return t.String()
}

func locOverview(w io.Writer, l *backend.Location) {
	s := l.Stats

	fmt.Fprintf(w, "%s\t%s\t%s\t%0.1f\t%0.3f\t%0.3f\t%s\t%s\n",
		l.Id,
		l.Hostname,
		l.Path,
		s.RequestsPerSecond(),
		float64(s.Latency.Q95)/1000.0,
		float64(s.Latency.Q99)/1000.0,
		statusCodesToString(&s),
		errRateToString(s.NetErrorRate()))
}

func endpointOverview(w io.Writer, e *backend.Endpoint) {
	s := e.Stats

	fmt.Fprintf(w, "%s\t%s\t%s\t%0.1f\t%0.3f\t%0.3f\t%s\t%s\n",
		e.UpstreamId,
		e.Id,
		e.Url,
		s.RequestsPerSecond(),
		float64(s.Latency.Q95)/1000.0,
		float64(s.Latency.Q99)/1000.0,
		statusCodesToString(&s),
		errRateToString(s.NetErrorRate()))
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
	return cmpStats(&s.locs[i].Stats, &s.locs[j].Stats)
}

type endpointSorter struct {
	es []*backend.Endpoint
}

func (s *endpointSorter) Len() int {
	return len(s.es)
}

func (s *endpointSorter) Swap(i, j int) {
	s.es[i], s.es[j] = s.es[j], s.es[i]
}

func (s *endpointSorter) Less(i, j int) bool {
	return cmpStats(&s.es[i].Stats, &s.es[j].Stats)
}

func cmpStats(s1, s2 *backend.RoundTripStats) bool {
	if s1.NetErrorRate() != 0 || s2.NetErrorRate() != 0 {
		return s1.NetErrorRate() > s2.NetErrorRate()
	}

	return (s1.Latency.Q99 > s2.Latency.Q99 || s1.Latency.Q95 > s2.Latency.Q95 || s1.Counters.Total > s2.Counters.Total)
}

func errRateToString(r float64) string {
	failRateS := fmt.Sprintf("%0.2f", r)
	if r != 0 {
		return goterm.Color(failRateS, goterm.RED)
	} else {
		return goterm.Color(failRateS, goterm.GREEN)
	}
}

func statusCodesToString(s *backend.RoundTripStats) string {
	codes := make([]string, 0, len(s.Counters.StatusCodes))
	if s.Counters.Total != 0 {
		for _, c := range s.Counters.StatusCodes {
			percent := 100 * (float64(c.Count) / float64(s.Counters.Total))
			out := fmt.Sprintf("%d: %0.2f", c.Code, percent)
			codes = append(codes, out)
		}
	}
	return strings.Join(codes, ", ")
}

func getColor(code int) int {
	if code < 300 {
		return goterm.GREEN
	} else if code < 500 {
		return goterm.YELLOW
	}
	return goterm.RED
}
