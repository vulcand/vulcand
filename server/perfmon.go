package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/backend"
)

const (
	counterBuckets         = 10
	counterResolution      = time.Second
	histMin                = 1
	histMax                = 3600000000       // 1 hour in microseconds
	histSignificantFigures = 2                // signigicant figures (1% precision)
	histBuckets            = 6                // number of sub-histograms in a rolling histogram
	histPeriod             = 10 * time.Second // roll time
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

	m.recordBucketMetrics(e.location.GetUniqueId().String(), m.locations, a)
	m.recordBucketMetrics(e.location.Upstream.Id, m.upstreams, a)
	m.recordBucketMetrics(e.endpoint.GetUniqueId().String(), m.endpoints, a)
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

func (m *perfMon) recordBucketMetrics(id string, ms map[string]*metricsBucket, a request.Attempt) {
	m.m.Lock()
	defer m.m.Unlock()

	if b, err := m.getBucket(id, ms); err == nil {
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

func (m *perfMon) getBucket(id string, ms map[string]*metricsBucket) (*metricsBucket, error) {
	if b, ok := ms[id]; ok {
		return b, nil
	}
	h, err := metrics.NewRollingHistogram(
		// this will create subhistograms
		metrics.NewHDRHistogramFn(histMin, histMax, histSignificantFigures),
		// number of buckets in a rolling histogram
		histBuckets,
		// rolling period for a histogram
		histPeriod,
		m.timeProvider)
	if err != nil {
		return nil, err
	}

	newCounter := func() (*metrics.RollingCounter, error) {
		return metrics.NewRollingCounter(counterBuckets, counterResolution, m.timeProvider)
	}

	netErrors, err := newCounter()
	if err != nil {
		return nil, err
	}

	total, err := newCounter()
	if err != nil {
		return nil, err
	}

	b := &metricsBucket{
		total:       total,
		netErrors:   netErrors,
		newCounter:  newCounter,
		statusCodes: make(map[int]*metrics.RollingCounter),
		histogram:   h,
	}
	ms[id] = b
	return b, nil
}

// metricBucket holds common metrics collected for every part that serves requests.
type metricsBucket struct {
	total       *metrics.RollingCounter
	netErrors   *metrics.RollingCounter
	newCounter  metrics.NewRollingCounterFn
	statusCodes map[int]*metrics.RollingCounter
	histogram   metrics.RollingHistogram
}

func (m *metricsBucket) recordMetrics(a request.Attempt) {
	m.total.Inc()
	m.recordNetError(a)
	m.recordLatency(a)
	m.recordStatusCode(a)
}

func (m *metricsBucket) recordNetError(a request.Attempt) {
	if metrics.IsNetworkError(a) {
		m.netErrors.Inc()
	}
}

func (m *metricsBucket) recordLatency(a request.Attempt) {
	if err := m.histogram.RecordValues(int64(a.GetDuration()/time.Microsecond), 1); err != nil {
		log.Errorf("Failed to record latency: %v", err)
	}
}

func (m *metricsBucket) recordStatusCode(a request.Attempt) {
	if a.GetResponse() == nil {
		return
	}
	statusCode := a.GetResponse().StatusCode
	if c, ok := m.statusCodes[statusCode]; ok {
		c.Inc()
		return
	}
	c, err := m.newCounter()
	if err != nil {
		log.Errorf("failed to create a counter: %v", err)
		return
	}
	c.Inc()
	m.statusCodes[statusCode] = c
}

func (m *metricsBucket) getStats() (*backend.RoundTripStats, error) {
	h, err := m.histogram.Merged()
	if err != nil {
		return nil, err
	}

	sc := make([]backend.StatusCode, 0, len(m.statusCodes))
	for k, v := range m.statusCodes {
		sc = append(sc, backend.StatusCode{Code: k, Count: v.Count()})
	}

	return &backend.RoundTripStats{
		Counters: backend.Counters{
			NetErrors:   m.netErrors.Count(),
			Total:       m.total.Count(),
			Period:      m.total.GetWindowSize(),
			StatusCodes: sc,
		},
		Latency: backend.NewBrackets(h),
	}, nil
}
