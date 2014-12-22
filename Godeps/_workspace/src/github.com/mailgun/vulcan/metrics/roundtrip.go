package metrics

import (
	"time"

	"github.com/mailgun/log"
	"github.com/mailgun/timetools"
	"github.com/mailgun/vulcan/request"
)

// RoundTripMetrics provides aggregated performance metrics for HTTP requests processing
// such as round trip latency, response codes counters network error and total requests.
// all counters are collected as rolling window counters with defined precision, histograms
// are a rolling window histograms with defined precision as well.
// See RoundTripOptions for more detail on parameters.
type RoundTripMetrics struct {
	o           *RoundTripOptions
	total       *RollingCounter
	netErrors   *RollingCounter
	statusCodes map[int]*RollingCounter
	histogram   RollingHistogram
}

type RoundTripOptions struct {
	// CounterBuckets - how many buckets to allocate for rolling counter. Defaults to 10 buckets.
	CounterBuckets int
	// CounterResolution specifies the resolution for a single bucket
	// (e.g. time.Second means that bucket will be counted for a second).
	// defaults to time.Second
	CounterResolution time.Duration
	// HistMin - minimum non 0 value for a histogram (default 1)
	HistMin int64
	// HistMax - maximum value that can be recorded for a histogram (default 3,600,000,000)
	HistMax int64
	// HistSignificantFigures - defines precision for a value. e.g. 3 - 0.1%X precision, default is 2 - 1% precision for X
	HistSignificantFigures int
	// HistBuckets - how many sub histogram to keep in a rolling histogram, default is 6
	HistBuckets int
	// HistPeriod - rotation period for a histogram, default is 10 seconds
	HistPeriod time.Duration
	// TimeProvider - to provide time provider in tests, default is RealTime
	TimeProvider timetools.TimeProvider
}

// NewRoundTripMetrics returns new instance of metrics collector.
func NewRoundTripMetrics(o RoundTripOptions) (*RoundTripMetrics, error) {
	o = setDefaults(o)

	h, err := NewRollingHistogram(
		// this will create subhistograms
		NewHDRHistogramFn(o.HistMin, o.HistMax, o.HistSignificantFigures),
		// number of buckets in a rolling histogram
		o.HistBuckets,
		// rolling period for a histogram
		o.HistPeriod,
		o.TimeProvider)
	if err != nil {
		return nil, err
	}

	m := &RoundTripMetrics{
		statusCodes: make(map[int]*RollingCounter),
		histogram:   h,
		o:           &o,
	}

	netErrors, err := m.newCounter()
	if err != nil {
		return nil, err
	}

	total, err := m.newCounter()
	if err != nil {
		return nil, err
	}

	m.netErrors = netErrors
	m.total = total
	return m, nil
}

// GetOptions returns settings used for this instance
func (m *RoundTripMetrics) GetOptions() *RoundTripOptions {
	return m.o
}

// GetNetworkErrorRatio calculates the amont of network errors such as time outs and dropped connection
// that occured in the given time window compared to the total requests count.
func (m *RoundTripMetrics) GetNetworkErrorRatio() float64 {
	if m.total.Count() == 0 {
		return 0
	}
	return float64(m.netErrors.Count()) / float64(m.total.Count())
}

// GetResponseCodeRatio calculates ratio of count(startA to endA) / count(startB to endB)
func (m *RoundTripMetrics) GetResponseCodeRatio(startA, endA, startB, endB int) float64 {
	a := int64(0)
	b := int64(0)
	for code, v := range m.statusCodes {
		if code < endA && code >= startA {
			a += v.Count()
		}
		if code < endB && code >= startB {
			b += v.Count()
		}
	}
	if b != 0 {
		return float64(a) / float64(b)
	}
	return 0
}

// RecordMetrics updates internal metrics collection based on the data from passed request.
func (m *RoundTripMetrics) RecordMetrics(a request.Attempt) {
	m.total.Inc()
	m.recordNetError(a)
	m.recordLatency(a)
	m.recordStatusCode(a)
}

// GetTotalCount returns total count of processed requests collected.
func (m *RoundTripMetrics) GetTotalCount() int64 {
	return m.total.Count()
}

// GetNetworkErrorCount returns total count of processed requests observed
func (m *RoundTripMetrics) GetNetworkErrorCount() int64 {
	return m.netErrors.Count()
}

// GetStatusCodesCounts returns map with counts of the response codes
func (m *RoundTripMetrics) GetStatusCodesCounts() map[int]int64 {
	sc := make(map[int]int64)
	for k, v := range m.statusCodes {
		if v.Count() != 0 {
			sc[k] = v.Count()
		}
	}
	return sc
}

// GetLatencyHistogram computes and returns resulting histogram with latencies observed.
func (m *RoundTripMetrics) GetLatencyHistogram() (Histogram, error) {
	return m.histogram.Merged()
}

func (m *RoundTripMetrics) Reset() {
	m.histogram.Reset()
	m.total.Reset()
	m.netErrors.Reset()
	m.statusCodes = make(map[int]*RollingCounter)
}

func (m *RoundTripMetrics) newCounter() (*RollingCounter, error) {
	return NewRollingCounter(m.o.CounterBuckets, m.o.CounterResolution, m.o.TimeProvider)
}

func (m *RoundTripMetrics) recordNetError(a request.Attempt) {
	if IsNetworkError(a) {
		m.netErrors.Inc()
	}
}

func (m *RoundTripMetrics) recordLatency(a request.Attempt) {
	if err := m.histogram.RecordLatencies(a.GetDuration(), 1); err != nil {
		log.Errorf("Failed to record latency: %v", err)
	}
}

func (m *RoundTripMetrics) recordStatusCode(a request.Attempt) {
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

const (
	counterBuckets         = 10
	counterResolution      = time.Second
	histMin                = 1
	histMax                = 3600000000       // 1 hour in microseconds
	histSignificantFigures = 2                // signigicant figures (1% precision)
	histBuckets            = 6                // number of sub-histograms in a rolling histogram
	histPeriod             = 10 * time.Second // roll time
)

func setDefaults(o RoundTripOptions) RoundTripOptions {
	if o.CounterBuckets == 0 {
		o.CounterBuckets = counterBuckets
	}
	if o.CounterResolution == 0 {
		o.CounterResolution = time.Second
	}
	if o.HistMin == 0 {
		o.HistMin = histMin
	}
	if o.HistMax == 0 {
		o.HistMax = histMax
	}
	if o.HistBuckets == 0 {
		o.HistBuckets = histBuckets
	}
	if o.HistSignificantFigures == 0 {
		o.HistSignificantFigures = histSignificantFigures
	}
	if o.HistPeriod == 0 {
		o.HistPeriod = histPeriod
	}
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	return o
}
