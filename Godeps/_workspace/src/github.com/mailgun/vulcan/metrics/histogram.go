package metrics

import (
	"fmt"
	"time"

	"github.com/codahale/hdrhistogram"
	"github.com/mailgun/timetools"
)

type Histogram interface {
	// Returns latency at quantile with microsecond precision
	LatencyAtQuantile(float64) time.Duration
	// Records latencies with microsecond precision
	RecordLatencies(d time.Duration, n int64) error

	ValueAtQuantile(q float64) int64
	RecordValues(v, n int64) error
	// Merge updates this histogram with values of another histogram
	Merge(Histogram) error
	// Resets state of the histogram
	Reset()
}

// RollingHistogram holds multiple histograms and rotates every period.
// It provides resulting histogram as a result of a call of 'Merged' function.
type RollingHistogram interface {
	RecordValues(v, n int64) error
	RecordLatencies(d time.Duration, n int64) error
	Merged() (Histogram, error)
	Reset()
}

// NewHistogramFn is a constructor that can be passed to NewRollingHistogram
type NewHistogramFn func() (Histogram, error)

// NewHDRHistogramFn creates a constructor of HDR histograms with predefined parameters.
func NewHDRHistogramFn(low, high int64, sigfigs int) NewHistogramFn {
	return func() (Histogram, error) {
		return NewHDRHistogram(low, high, sigfigs)
	}
}

type rollingHistogram struct {
	maker        NewHistogramFn
	idx          int
	lastRoll     time.Time
	period       time.Duration
	buckets      []Histogram
	timeProvider timetools.TimeProvider
}

func NewRollingHistogram(maker NewHistogramFn, bucketCount int, period time.Duration, timeProvider timetools.TimeProvider) (RollingHistogram, error) {
	buckets := make([]Histogram, bucketCount)
	for i := range buckets {
		h, err := maker()
		if err != nil {
			return nil, err
		}
		buckets[i] = h
	}

	return &rollingHistogram{
		maker:        maker,
		buckets:      buckets,
		period:       period,
		timeProvider: timeProvider,
	}, nil
}

func (r *rollingHistogram) Reset() {
	r.idx = 0
	r.lastRoll = r.timeProvider.UtcNow()
	for _, b := range r.buckets {
		b.Reset()
	}
}

func (r *rollingHistogram) rotate() {
	r.idx = (r.idx + 1) % len(r.buckets)
	r.buckets[r.idx].Reset()
}

func (r *rollingHistogram) Merged() (Histogram, error) {
	m, err := r.maker()
	if err != nil {
		return m, err
	}
	for _, h := range r.buckets {
		if m.Merge(h); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (r *rollingHistogram) getHist() Histogram {
	if r.timeProvider.UtcNow().Sub(r.lastRoll) >= r.period {
		r.rotate()
		r.lastRoll = r.timeProvider.UtcNow()
	}
	return r.buckets[r.idx]
}

func (r *rollingHistogram) RecordLatencies(v time.Duration, n int64) error {
	return r.getHist().RecordLatencies(v, n)
}

func (r *rollingHistogram) RecordValues(v, n int64) error {
	return r.getHist().RecordValues(v, n)
}

type HDRHistogram struct {
	// lowest trackable value
	low int64
	// highest trackable value
	high int64
	// significant figures
	sigfigs int

	h *hdrhistogram.Histogram
}

func NewHDRHistogram(low, high int64, sigfigs int) (h *HDRHistogram, err error) {
	defer func() {
		if msg := recover(); msg != nil {
			err = fmt.Errorf("%s", msg)
		}
	}()

	hdr := hdrhistogram.New(low, high, sigfigs)
	h = &HDRHistogram{
		low:     low,
		high:    high,
		sigfigs: sigfigs,
		h:       hdr,
	}
	return h, err
}

// Returns latency at quantile with microsecond precision
func (h *HDRHistogram) LatencyAtQuantile(q float64) time.Duration {
	return time.Duration(h.ValueAtQuantile(q)) * time.Microsecond
}

// Records latencies with microsecond precision
func (h *HDRHistogram) RecordLatencies(d time.Duration, n int64) error {
	return h.RecordValues(int64(d/time.Microsecond), n)
}

func (h *HDRHistogram) Reset() {
	h.h.Reset()
}

func (h *HDRHistogram) ValueAtQuantile(q float64) int64 {
	return h.h.ValueAtQuantile(q)
}

func (h *HDRHistogram) RecordValues(v, n int64) error {
	return h.h.RecordValues(v, n)
}

func (h *HDRHistogram) Merge(o Histogram) error {
	other, ok := o.(*HDRHistogram)
	if !ok {
		return fmt.Errorf("can merge only with other HDRHistogram, got %T", o)
	}

	h.h.Merge(other.h)
	return nil
}
