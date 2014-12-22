package server

import (
	"fmt"
	"sort"
	"sync"

	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcand/backend"
)

// perfMon stands for performance monitor, it is observer that watches realtime metrics
// for locations, endpoints and upstreams
type perfMon struct {
	m            *sync.RWMutex
	locations    map[string]*metricsBucket
	endpoints    map[string]*metricsBucket
	upstreams    map[string]*metricsBucket
	timeProvider timetools.TimeProvider
}

func newPerfMon(timeProvider timetools.TimeProvider) *perfMon {
	return &perfMon{
		m:            &sync.RWMutex{},
		locations:    make(map[string]*metricsBucket),
		endpoints:    make(map[string]*metricsBucket),
		upstreams:    make(map[string]*metricsBucket),
		timeProvider: timeProvider,
	}
}

func (m *perfMon) getTopLocations(hostname, upstreamId string) ([]*backend.Location, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	locations := []*backend.Location{}
	for _, m := range m.locations {
		l := m.endpoint.location
		if hostname != "" && l.Hostname != hostname {
			continue
		}
		if upstreamId != "" && l.Upstream.Id != upstreamId {
			continue
		}
		if s, err := m.getStats(); err == nil {
			l.Stats = *s
		}
		locations = append(locations, l)
	}
	sort.Sort(&locSorter{locs: locations})
	return locations, nil
}

func (m *perfMon) getTopEndpoints(upstreamId string) ([]*backend.Endpoint, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	endpoints := []*backend.Endpoint{}
	for _, m := range m.endpoints {
		e := m.endpoint.endpoint
		if upstreamId != "" && e.UpstreamId != upstreamId {
			continue
		}
		if s, err := m.getStats(); err == nil {
			e.Stats = *s
		}
		endpoints = append(endpoints, e)
	}
	sort.Sort(&endpointSorter{es: endpoints})
	return endpoints, nil
}

func (m *perfMon) resetLocationStats(l *backend.Location) error {
	m.m.Lock()
	defer m.m.Unlock()

	b, err := m.findBucket(l.GetUniqueId().String(), m.locations)
	if err != nil {
		return err
	}

	return b.resetStats()
}

func (m *perfMon) getLocationStats(l *backend.Location) (*backend.RoundTripStats, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	b, err := m.findBucket(l.GetUniqueId().String(), m.locations)
	if err != nil {
		return nil, err
	}

	return b.getStats()
}

func (m *perfMon) getEndpointStats(e *backend.Endpoint) (*backend.RoundTripStats, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	b, err := m.findBucket(e.GetUniqueId().String(), m.endpoints)
	if err != nil {
		return nil, err
	}
	return b.getStats()
}

func (m *perfMon) getUpstreamStats(u *backend.Upstream) (*backend.RoundTripStats, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	b, err := m.findBucket(u.Id, m.upstreams)
	if err != nil {
		return nil, err
	}
	return b.getStats()
}

func (m *perfMon) ObserveRequest(r request.Request) {
}

func (m *perfMon) ObserveResponse(r request.Request, a request.Attempt) {
	if a == nil || a.GetEndpoint() == nil {
		return
	}

	e, ok := a.GetEndpoint().(*muxEndpoint)
	if !ok {
		log.Errorf("Unknown endpoint type %T", a.GetEndpoint())
		return
	}

	m.recordBucketMetrics(e.location.GetUniqueId().String(), m.locations, a, e)
	m.recordBucketMetrics(e.location.Upstream.Id, m.upstreams, a, e)
	m.recordBucketMetrics(e.endpoint.GetUniqueId().String(), m.endpoints, a, e)
}

func (m *perfMon) deleteLocation(key backend.LocationKey) {
	m.deleteBucket(key.String(), m.locations)
}

func (m *perfMon) deleteEndpoint(key backend.EndpointKey) {
	m.deleteBucket(key.String(), m.endpoints)
}

func (m *perfMon) deleteUpstream(up backend.UpstreamKey) {
	m.deleteBucket(up.String(), m.upstreams)
	for k, _ := range m.endpoints {
		eKey := backend.MustParseEndpointKey(k)
		if eKey.UpstreamId == up.String() {
			m.deleteBucket(eKey.String(), m.endpoints)
		}
	}
}

func (m *perfMon) recordBucketMetrics(id string, ms map[string]*metricsBucket, a request.Attempt, e *muxEndpoint) {
	m.m.Lock()
	defer m.m.Unlock()

	if b, err := m.getBucket(id, ms, e); err == nil {
		b.recordMetrics(a)
	} else {
		log.Errorf("failed to get bucket for %v, error: %v", id, err)
	}
}

func (m *perfMon) deleteBucket(id string, ms map[string]*metricsBucket) {
	m.m.Lock()
	defer m.m.Unlock()

	delete(ms, id)
}

func (m *perfMon) findBucket(id string, ms map[string]*metricsBucket) (*metricsBucket, error) {
	if b, ok := ms[id]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("bucket %s not found", id)
}

func (m *perfMon) getBucket(id string, ms map[string]*metricsBucket, e *muxEndpoint) (*metricsBucket, error) {
	if b, ok := ms[id]; ok {
		return b, nil
	}
	mt, err := metrics.NewRoundTripMetrics(metrics.RoundTripOptions{TimeProvider: m.timeProvider})
	if err != nil {
		return nil, err
	}
	b := &metricsBucket{
		endpoint: e,
		metrics:  mt,
	}
	ms[id] = b
	return b, nil
}

// metricBucket holds common metrics collected for every part that serves requests.
type metricsBucket struct {
	endpoint *muxEndpoint
	metrics  *metrics.RoundTripMetrics
}

func (m *metricsBucket) recordMetrics(a request.Attempt) {
	m.metrics.RecordMetrics(a)
}

func (m *metricsBucket) resetStats() error {
	m.metrics.Reset()
	return nil
}

func (m *metricsBucket) getStats() (*backend.RoundTripStats, error) {
	return backend.NewRoundTripStats(m.metrics)
}

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
