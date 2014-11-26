// Tokenbucket based request rate limiter
package tokenbucket

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/ttlmap"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

const DefaultCapacity = 65536

// RateSet maintains a set of rates. It can contain only one rate per period at a time.
type RateSet struct {
	m map[time.Duration]*rate
}

// NewRateSet crates an empty `RateSet` instance.
func NewRateSet() *RateSet {
	rs := new(RateSet)
	rs.m = make(map[time.Duration]*rate)
	return rs
}

// Add adds a rate to the set. If there is a rate with the same period in the
// set then the new rate overrides the old one.
func (rs *RateSet) Add(period time.Duration, average int64, burst int64) error {
	if period <= 0 {
		return fmt.Errorf("Invalid period: %v", period)
	}
	if average <= 0 {
		return fmt.Errorf("Invalid average: %v", average)
	}
	if burst <= 0 {
		return fmt.Errorf("Invalid burst: %v", burst)
	}
	rs.m[period] = &rate{period, average, burst}
	return nil
}

func (rs *RateSet) String() string {
	return fmt.Sprint(rs.m)
}

// ConfigMapperFn is a mapper function that is used by the `TokenLimiter`
// middleware to retrieve `RateSet` from HTTP requests.
type ConfigMapperFn func(r request.Request) (*RateSet, error)

// TokenLimiter implements rate limiting middleware.
type TokenLimiter struct {
	defaultRates *RateSet
	mapper       limit.MapperFn
	configMapper ConfigMapperFn
	clock        timetools.TimeProvider
	mutex        sync.Mutex
	bucketSets   *ttlmap.TtlMap
}

// NewLimiter constructs a `TokenLimiter` middleware instance.
func NewLimiter(defaultRates *RateSet, capacity int, mapper limit.MapperFn, configMapper ConfigMapperFn, clock timetools.TimeProvider) (*TokenLimiter, error) {
	if defaultRates == nil || len(defaultRates.m) == 0 {
		return nil, fmt.Errorf("Provide default rates")
	}
	if mapper == nil {
		return nil, fmt.Errorf("Provide mapper function")
	}

	// Set default values for optional fields.
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	if clock == nil {
		clock = &timetools.RealTime{}
	}

	bucketSets, err := ttlmap.NewMapWithProvider(DefaultCapacity, clock)
	if err != nil {
		return nil, err
	}

	return &TokenLimiter{
		defaultRates: defaultRates,
		mapper:       mapper,
		configMapper: configMapper,
		clock:        clock,
		bucketSets:   bucketSets,
	}, nil
}

// DefaultRates returns the default rate set of the limiter. The only reason to
// Provide this method is to facilitate testing.
func (tl *TokenLimiter) DefaultRates() *RateSet {
	defaultRates := NewRateSet()
	for _, r := range tl.defaultRates.m {
		defaultRates.Add(r.period, r.average, r.burst)
	}
	return defaultRates
}

func (tl *TokenLimiter) ProcessRequest(r request.Request) (*http.Response, error) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	token, amount, err := tl.mapper(r)
	if err != nil {
		return nil, err
	}

	effectiveRates := tl.effectiveRates(r)
	bucketSetI, exists := tl.bucketSets.Get(token)
	var bucketSet *tokenBucketSet

	if exists {
		bucketSet = bucketSetI.(*tokenBucketSet)
		bucketSet.update(effectiveRates)
	} else {
		bucketSet = newTokenBucketSet(effectiveRates, tl.clock)
		// We set ttl as 10 times rate period. E.g. if rate is 100 requests/second per client ip
		// the counters for this ip will expire after 10 seconds of inactivity
		tl.bucketSets.Set(token, bucketSet, int(bucketSet.maxPeriod/time.Second)*10+1)
	}

	delay, err := bucketSet.consume(amount)
	if err != nil {
		return nil, err
	}
	if delay > 0 {
		return netutils.NewTextResponse(r.GetHttpRequest(), errors.StatusTooManyRequests, "Too many requests"), nil
	}
	return nil, nil
}

func (tl *TokenLimiter) ProcessResponse(r request.Request, a request.Attempt) {
}

// effectiveRates retrieves rates to be applied to the request.
func (tl *TokenLimiter) effectiveRates(r request.Request) *RateSet {
	// If configuration mapper is not specified for this instance, then return
	// the default bucket specs.
	if tl.configMapper == nil {
		return tl.defaultRates
	}

	rates, err := tl.configMapper(r)
	if err != nil {
		log.Errorf("Failed to retrieve rates: %v", err)
		return tl.defaultRates
	}

	// If the returned rate set is empty then used the default one.
	if len(rates.m) == 0 {
		return tl.defaultRates
	}

	return rates
}
