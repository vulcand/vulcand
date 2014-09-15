// Tokenbucket based request rate limiter
package tokenbucket

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/ttlmap"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"net/http"
	"sync"
	"time"
)

type TokenLimiter struct {
	buckets *ttlmap.TtlMap
	mutex   *sync.Mutex
	options Options
	mapper  limit.MapperFn
	rate    Rate
}

type Options struct {
	Rate         Rate  // Average allowed rate
	Burst        int64 // Burst size
	Capacity     int   // Overall capacity (maximum sumultaneuously active tokens)
	Mapper       limit.MapperFn
	TimeProvider timetools.TimeProvider
}

func NewTokenLimiter(mapper limit.MapperFn, rate Rate) (*TokenLimiter, error) {
	return NewTokenLimiterWithOptions(mapper, rate, Options{})
}

func NewTokenLimiterWithOptions(mapper limit.MapperFn, rate Rate, o Options) (*TokenLimiter, error) {
	if mapper == nil {
		return nil, fmt.Errorf("Provide mapper function")
	}
	options, err := parseOptions(o)
	if err != nil {
		return nil, err
	}
	buckets, err := ttlmap.NewMapWithProvider(options.Capacity, options.TimeProvider)
	if err != nil {
		return nil, err
	}

	return &TokenLimiter{
		rate:    rate,
		mapper:  mapper,
		options: options,
		mutex:   &sync.Mutex{},
		buckets: buckets,
	}, nil
}

func (tl *TokenLimiter) GetRate() Rate {
	return tl.rate
}

func (tl *TokenLimiter) GetBurst() int64 {
	return tl.options.Burst
}

func (tl *TokenLimiter) GetCapacity() int {
	return tl.options.Capacity
}

func (tl *TokenLimiter) ProcessRequest(r request.Request) (*http.Response, error) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	token, amount, err := tl.mapper(r)
	if err != nil {
		return nil, err
	}

	bucketI, exists := tl.buckets.Get(token)
	if !exists {
		bucketI, err = NewTokenBucket(tl.rate, tl.options.Burst+1, tl.options.TimeProvider)
		if err != nil {
			return nil, err
		}
		// We set ttl as 10 times rate period. E.g. if rate is 100 requests/second per client ip
		// the counters for this ip will expire after 10 seconds of inactivity
		tl.buckets.Set(token, bucketI, int(tl.rate.Period/time.Second)*10+1)
	}
	bucket := bucketI.(*TokenBucket)
	delay, err := bucket.Consume(amount)
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

// Check arguments and initialize defaults
func parseOptions(o Options) (Options, error) {
	if o.Capacity <= 0 {
		o.Capacity = DefaultCapacity
	}
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	return o, nil
}

const DefaultCapacity = 65536
