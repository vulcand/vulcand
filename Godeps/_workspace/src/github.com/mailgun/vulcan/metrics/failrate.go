// In memory request performance metrics
package metrics

import (
	"fmt"
	"time"

	"github.com/mailgun/timetools"
	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcan/request"
)

type FailRateMeter interface {
	GetRate() float64
	IsReady() bool
	GetWindowSize() time.Duration
	middleware.Observer
}

// Predicate that helps to see if the attempt resulted in error
type FailPredicate func(request.Attempt) bool

func IsNetworkError(attempt request.Attempt) bool {
	return attempt != nil && attempt.GetError() != nil
}

// Calculates various performance metrics about the endpoint using counters of the predefined size
type RollingMeter struct {
	endpoint endpoint.Endpoint
	isError  FailPredicate

	errors    *RollingCounter
	successes *RollingCounter
}

func NewRollingMeter(endpoint endpoint.Endpoint, buckets int, resolution time.Duration, timeProvider timetools.TimeProvider, isError FailPredicate) (*RollingMeter, error) {
	if endpoint == nil {
		return nil, fmt.Errorf("Select an endpoint")
	}
	if isError == nil {
		isError = IsNetworkError
	}

	e, err := NewRollingCounter(buckets, resolution, timeProvider)
	if err != nil {
		return nil, err
	}

	s, err := NewRollingCounter(buckets, resolution, timeProvider)
	if err != nil {
		return nil, err
	}

	return &RollingMeter{
		endpoint:  endpoint,
		errors:    e,
		successes: s,
		isError:   isError,
	}, nil
}

func (r *RollingMeter) Reset() {
	r.errors.Reset()
	r.successes.Reset()
}

func (r *RollingMeter) IsReady() bool {
	return r.errors.countedBuckets+r.successes.countedBuckets >= len(r.errors.values)
}

func (r *RollingMeter) SuccessCount() int64 {
	return r.successes.Count()
}

func (r *RollingMeter) FailureCount() int64 {
	return r.errors.Count()
}

func (r *RollingMeter) Resolution() time.Duration {
	return r.errors.Resolution()
}

func (r *RollingMeter) Buckets() int {
	return r.errors.Buckets()
}

func (r *RollingMeter) GetWindowSize() time.Duration {
	return r.errors.GetWindowSize()
}

func (r *RollingMeter) ProcessedCount() int64 {
	return r.SuccessCount() + r.FailureCount()
}

func (r *RollingMeter) GetRate() float64 {
	success := r.SuccessCount()
	failure := r.FailureCount()
	// No data, return ok
	if success+failure == 0 {
		return 0
	}
	return float64(failure) / float64(success+failure)
}

func (r *RollingMeter) ObserveRequest(request.Request) {
}

func (r *RollingMeter) ObserveResponse(req request.Request, lastAttempt request.Attempt) {
	if lastAttempt == nil || lastAttempt.GetEndpoint() != r.endpoint {
		return
	}

	if r.isError(lastAttempt) {
		r.errors.Inc()
	} else {
		r.successes.Inc()
	}
}

type TestMeter struct {
	Rate       float64
	NotReady   bool
	WindowSize time.Duration
}

func (tm *TestMeter) GetWindowSize() time.Duration {
	return tm.WindowSize
}

func (tm *TestMeter) IsReady() bool {
	return !tm.NotReady
}

func (tm *TestMeter) GetRate() float64 {
	return tm.Rate
}

func (em *TestMeter) ObserveRequest(r request.Request) {
}

func (em *TestMeter) ObserveResponse(r request.Request, lastAttempt request.Attempt) {
}
