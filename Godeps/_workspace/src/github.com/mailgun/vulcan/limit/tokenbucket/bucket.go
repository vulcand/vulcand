package tokenbucket

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	"time"
)

type Rate struct {
	Units  int64
	Period time.Duration
}

// Implements token bucket rate limiting algorithm (http://en.wikipedia.org/wiki/Token_bucket)
// and is used by rate limiters to implement various rate limiting strategies
type TokenBucket struct {
	// Maximum amount of tokens available at given time (controls burst rate)
	maxTokens int64
	// Specifies the period of the rate
	refillPeriod time.Duration
	// Current value of tokens
	tokens int64
	// Interface that gives current time (so tests can override)
	timeProvider timetools.TimeProvider
	lastRefill   time.Time
}

func NewTokenBucket(rate Rate, maxTokens int64, timeProvider timetools.TimeProvider) (*TokenBucket, error) {
	if rate.Period == 0 || rate.Units == 0 {
		return nil, fmt.Errorf("Invalid rate: %v", rate)
	}
	if maxTokens <= 0 {
		return nil, fmt.Errorf("Invalid maxTokens, should be >0: %d", maxTokens)
	}
	if timeProvider == nil {
		return nil, fmt.Errorf("Supply time provider")
	}
	return &TokenBucket{
		refillPeriod: time.Duration(int64(rate.Period) / rate.Units),
		maxTokens:    maxTokens, // in case of maxBurst is 0, maxTokens available at should be 1
		timeProvider: timeProvider,
		lastRefill:   timeProvider.UtcNow(),
		tokens:       maxTokens,
	}, nil
}

// In case if there's enough tokens, consumes tokens and returns 0, nil
// In case if tokens to consume is larger than max burst returns -1, error
// In case if there's not enough tokens, returns time to wait till refill
func (tb *TokenBucket) Consume(tokens int64) (time.Duration, error) {
	tb.refill()
	if tokens > tb.maxTokens {
		return -1, fmt.Errorf("Requested tokens larger than max tokens")
	}
	if tb.tokens < tokens {
		return tb.timeToRefill(tokens), nil
	}
	tb.tokens -= tokens
	return 0, nil
}

// Returns the time after the capacity of tokens will reach the
func (tb *TokenBucket) timeToRefill(tokens int64) time.Duration {
	missingTokens := tokens - tb.tokens
	return time.Duration(missingTokens) * tb.refillPeriod
}

func (tb *TokenBucket) refill() {
	now := tb.timeProvider.UtcNow()
	timePassed := now.Sub(tb.lastRefill)

	tokens := tb.tokens + int64(timePassed/tb.refillPeriod)
	// If we haven't added any tokens that means that not enough time has passed,
	// in this case do not adjust last refill checkpoint, otherwise it will be
	// always moving in time in case of frequent requests that exceed the rate
	if tokens != tb.tokens {
		tb.lastRefill = now
		tb.tokens = tokens
	}
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
}
