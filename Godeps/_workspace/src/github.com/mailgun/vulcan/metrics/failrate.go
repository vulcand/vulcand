// In memory request performance metrics
package metrics

import (
	"fmt"
	timetools "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"time"
)

type FailRateMeter interface {
	GetRate() float64
	IsReady() bool
	Observer
}

// Predicate that helps to see if the attempt resulted in error
type FailPredicate func(Attempt) bool

func IsNetworkError(attempt Attempt) bool {
	return attempt != nil && attempt.GetError() != nil
}

// Calculates in memory failure rate of an endpoint using rolling window of a predefined size
type RollingMeter struct {
	lastUpdated    time.Time
	success        []int
	failure        []int
	endpoint       Endpoint
	buckets        int
	resolution     time.Duration
	isError        FailPredicate
	timeProvider   timetools.TimeProvider
	countedBuckets int // how many samples in different buckets have we collected so far
	lastBucket     int // last recorded bucket
}

func NewRollingMeter(endpoint Endpoint, buckets int, resolution time.Duration, timeProvider timetools.TimeProvider, isError FailPredicate) (*RollingMeter, error) {
	if buckets <= 0 {
		return nil, fmt.Errorf("Buckets should be >= 0")
	}
	if resolution < time.Second {
		return nil, fmt.Errorf("Resolution should be larger than a second")
	}
	if endpoint == nil {
		return nil, fmt.Errorf("Select an endpoint")
	}
	if isError == nil {
		isError = IsNetworkError
	}

	return &RollingMeter{
		endpoint:     endpoint,
		buckets:      buckets,
		resolution:   resolution,
		isError:      isError,
		timeProvider: timeProvider,
		success:      make([]int, buckets),
		failure:      make([]int, buckets),
		lastBucket:   -1,
	}, nil
}

func (em *RollingMeter) Reset() {
	em.lastBucket = -1
	em.countedBuckets = 0
	em.lastUpdated = time.Time{}
	for i, _ := range em.success {
		em.success[i] = 0
		em.failure[i] = 0
	}
}

func (em *RollingMeter) IsReady() bool {
	return em.countedBuckets >= em.buckets
}

func (em *RollingMeter) SuccessCount() int64 {
	em.cleanup(em.success)
	return em.sum(em.success)
}

func (em *RollingMeter) FailureCount() int64 {
	em.cleanup(em.failure)
	return em.sum(em.failure)
}

func (em *RollingMeter) Resolution() time.Duration {
	return em.resolution
}

func (em *RollingMeter) Buckets() int {
	return em.buckets
}

func (em *RollingMeter) WindowSize() time.Duration {
	return time.Duration(em.buckets) * em.resolution
}

func (em *RollingMeter) ProcessedCount() int64 {
	return em.SuccessCount() + em.FailureCount()
}

func (em *RollingMeter) GetRate() float64 {
	success := em.SuccessCount()
	failure := em.FailureCount()
	// No data, return ok
	if success+failure == 0 {
		return 0
	}
	return float64(failure) / float64(success+failure)
}

func (em *RollingMeter) ObserveRequest(r Request) {
}

func (em *RollingMeter) ObserveResponse(r Request, lastAttempt Attempt) {
	if lastAttempt == nil || lastAttempt.GetEndpoint() != em.endpoint {
		return
	}
	// Cleanup the data that was here in case if endpoint has been inactive for some time
	em.cleanup(em.failure)
	em.cleanup(em.success)

	if em.isError(lastAttempt) {
		em.incBucket(em.failure)
	} else {
		em.incBucket(em.success)
	}
}

// Returns the number in the moving window bucket that this slot occupies
func (em *RollingMeter) getBucket(t time.Time) int {
	return int(t.Truncate(em.resolution).Unix() % int64(em.buckets))
}

func (em *RollingMeter) incBucket(buckets []int) {
	now := em.timeProvider.UtcNow()
	bucket := em.getBucket(now)
	buckets[bucket] += 1
	em.lastUpdated = now
	// update usage stats if we haven't collected enough
	if !em.IsReady() {
		if em.lastBucket != bucket {
			em.lastBucket = bucket
			em.countedBuckets += 1
		}
	}
}

// Reset buckets that were not updated
func (em *RollingMeter) cleanup(buckets []int) {
	now := em.timeProvider.UtcNow()
	for i := 0; i < em.buckets; i++ {
		now = now.Add(time.Duration(-1*i) * em.resolution)
		if now.Truncate(em.resolution).After(em.lastUpdated.Truncate(em.resolution)) {
			buckets[em.getBucket(now)] = 0
		} else {
			break
		}
	}
}

func (em *RollingMeter) sum(buckets []int) int64 {
	out := int64(0)
	for _, v := range buckets {
		out += int64(v)
	}
	return out
}

type TestMeter struct {
	Rate     float64
	NotReady bool
}

func (tm *TestMeter) IsReady() bool {
	return !tm.NotReady
}

func (tm *TestMeter) GetRate() float64 {
	return tm.Rate
}

func (em *TestMeter) ObserveRequest(r Request) {
}

func (em *TestMeter) ObserveResponse(r Request, lastAttempt Attempt) {
}
