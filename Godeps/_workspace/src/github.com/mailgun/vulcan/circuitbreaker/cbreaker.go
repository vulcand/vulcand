// circuitbreaker package implements circuit breaker similar to  https://github.com/Netflix/Hystrix/wiki/How-it-Works
//
// Vulcand circuit breaker watches the error condtion to match
// after what it activates the fallback scenario, e.g. returns the response code
// or redirects the request to another location

// Circuit breakers start in the Standby state first, observing responses and watching location metrics.
//
// Once the Circuit breaker condition is met, it enters the "Tripped" state, where it activates fallback scenario
// for all requests during the FallbackDuration time period and reset the stats for the location.
//
// After FallbackDuration time period passes, Circuit breaker enters "Recovering" state, during that state it will
// start passing some traffic back to the endpoints, increasing the amount of passed requests using linear function:
//
//    allowedRequestsRatio = 0.5 * (Now() - StartRecovery())/RecoveryDuration
//
// Two scenarios are possible in the "Recovering" state:
// 1. Condition matches again, this will reset the state to "Tripped" and reset the timer.
// 2. Condition does not match, circuit breaker enters "Standby" state
//
// It is possible to define actions of transitions (Standby -> Tripped) and (Recovering -> Standby)
// using handlers 'OnTripped' and 'OnStandby', e.g. issuing webhook calls.
//
package circuitbreaker

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/threshold"
)

// cbState is the state of the circuit breaker
type cbState int

func (s cbState) String() string {
	switch s {
	case stateStandby:
		return "standby"
	case stateTripped:
		return "tripped"
	case stateRecovering:
		return "recovering"
	}
	return "undefined"
}

const (
	// CircuitBreaker is passing all requests and watching stats
	stateStandby = iota
	// CircuitBreaker activates fallback scenario for all requests
	stateTripped
	// CircuitBreaker passes some requests to go through, rejecting others
	stateRecovering
)

const (
	defaultFallbackDuration = 10 * time.Second
	defaultRecoveryDuration = 10 * time.Second
	defaultCheckPeriod      = 100 * time.Millisecond
)

// Options defines optional parameters for CircuitBreaker
type Options struct {
	// Check period is how frequently circuit breaker checks for the condition to match
	CheckPeriod time.Duration

	// FallbackDuration is a period for fallback scenario
	FallbackDuration time.Duration
	// RecoveryDuration is a period for recovery scenario
	RecoveryDuration time.Duration

	// TimeProvider is a interface to freeze time in tests
	TimeProvider timetools.TimeProvider

	// OnTripped defines action activated during (Standby->Tripped) transition
	OnTripped SideEffect

	// OnTripped defines action activated during (Recovering->Standby) transition
	OnStandby SideEffect
}

// CircuitBreaker is a middleware that implements circuit breaker pattern
type CircuitBreaker struct {
	m  *sync.RWMutex
	tm timetools.TimeProvider

	metrics *metrics.RoundTripMetrics

	condition threshold.Predicate
	duration  time.Duration

	state cbState
	until time.Time

	rc *ratioController

	onTripped SideEffect
	onStandby SideEffect

	recoveryDuration time.Duration
	fallbackDuration time.Duration

	checkPeriod time.Duration
	lastCheck   time.Time

	fallback middleware.Middleware
}

// New creates an new CircuitBreaker middleware
func New(condition threshold.Predicate, fallback middleware.Middleware, options Options) (*CircuitBreaker, error) {
	if condition == nil || fallback == nil {
		return nil, fmt.Errorf("provide non nil condition and fallback")
	}
	o, err := parseOptions(options)
	if err != nil {
		return nil, err
	}

	mt, err := metrics.NewRoundTripMetrics(metrics.RoundTripOptions{TimeProvider: o.TimeProvider})
	if err != nil {
		return nil, err
	}

	cb := &CircuitBreaker{
		tm:               o.TimeProvider,
		condition:        condition,
		fallback:         fallback,
		metrics:          mt,
		m:                &sync.RWMutex{},
		onTripped:        o.OnTripped,
		onStandby:        o.OnStandby,
		fallbackDuration: o.FallbackDuration,
		recoveryDuration: o.RecoveryDuration,
		checkPeriod:      o.CheckPeriod,
	}
	return cb, nil
}

// String returns log-friendly representation of the circuit breaker state
func (c *CircuitBreaker) String() string {
	switch c.state {
	case stateTripped, stateRecovering:
		return fmt.Sprintf("CircuitBreaker(state=%v, until=%v)", c.state, c.until)
	default:
		return fmt.Sprintf("CircuitBreaker(state=%v)", c.state)
	}
}

// ProcessRequest is called on every request to the endpoint, CircuitBreaker uses this feature
// to intercept the request if it's in Tripped state or slowly start passing the requests to endpoint
// if it's in the recovering state
func (c *CircuitBreaker) ProcessRequest(r request.Request) (*http.Response, error) {
	if c.isInactive() {
		c.markToRecordMetrics(r)
		return nil, nil
	}

	// Circuit breaker is in tripped or recovering state
	c.m.Lock()
	defer c.m.Unlock()

	log.Infof("%v is in error handling state", c)

	switch c.state {
	case stateStandby:
		// other goroutine has set it to standby state
		return nil, nil
	case stateTripped:
		// We have been in active state enough, enter recovering state
		if c.tm.UtcNow().After(c.until) {
			c.setRecovering(r)
		}
		return c.fallback.ProcessRequest(r)
	case stateRecovering:
		// We have been in recovering state enough, enter standby
		if c.tm.UtcNow().After(c.until) {
			c.setState(stateStandby, c.tm.UtcNow())
			return nil, nil
		}
		if c.rc.allowRequest() {
			// record metrics for allowed request
			c.markToRecordMetrics(r)
			return nil, nil
		}
		return c.fallback.ProcessRequest(r)
	}

	return nil, nil
}

func (c *CircuitBreaker) ProcessResponse(r request.Request, a request.Attempt) {
	// We should not record metrics for the requests intercepted by circuit breaker
	// otherwise our metrics would be incorrect
	if c.shouldRecordMetrics(r) {
		c.metrics.RecordMetrics(a)
	}

	// We check condition periodically as it may be quite expensive operation
	// (e.g. in case if we compute latencies from rolling histogram)
	if c.checkCondition(r) {
		c.setTripped(a.GetEndpoint())
	}
}

func (c *CircuitBreaker) isInactive() bool {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.state == stateStandby
}

// exec executes side effect
func (c *CircuitBreaker) exec(s SideEffect) {
	if s == nil {
		return
	}
	go func() {
		if err := s.Exec(); err != nil {
			log.Errorf("%v side effect failure: %v", c, err)
		}
	}()
}

func (c *CircuitBreaker) setState(new cbState, until time.Time) {
	log.Infof("%v setting state to %v, until %v", c, new, until)
	c.state = new
	c.until = until
	switch new {
	case stateTripped:
		c.exec(c.onTripped)
	case stateStandby:
		c.exec(c.onStandby)
	}
}

// cmpAndSetActive sets state only when current state is not tripped already
func (c *CircuitBreaker) setTripped(e endpoint.Endpoint) bool {
	c.m.Lock()
	defer c.m.Unlock()

	if c.state == stateTripped {
		log.Infof("%v skip set tripped", c)
		return false
	}

	c.setState(stateTripped, c.tm.UtcNow().Add(c.fallbackDuration))
	c.resetStats(e)
	return true
}

func (c *CircuitBreaker) timeToCheck() bool {
	c.m.RLock()
	defer c.m.RUnlock()
	return c.tm.UtcNow().After(c.lastCheck)
}

func (c *CircuitBreaker) checkCondition(r request.Request) bool {
	if !c.timeToCheck() {
		return false
	}

	c.m.Lock()
	defer c.m.Unlock()

	// Other goroutine could have updated the lastCheck variable before we grabbed mutex
	if !c.tm.UtcNow().After(c.lastCheck) {
		return false
	}
	c.lastCheck = c.tm.UtcNow().Add(c.checkPeriod)
	r.SetUserData(cbreakerMetrics, c.metrics)
	return c.condition(r)
}

func (c *CircuitBreaker) setRecovering(r request.Request) {
	c.setState(stateRecovering, c.tm.UtcNow().Add(c.recoveryDuration))
	c.rc = newRatioController(c.tm, c.recoveryDuration)
}

// resetStats is neccessary to start collecting fresh stats after fallback time passes.
func (c *CircuitBreaker) resetStats(e endpoint.Endpoint) {
	c.metrics.Reset()
}

func (c *CircuitBreaker) markToRecordMetrics(r request.Request) {
	r.SetUserData(cbreakerRecordMetrics, true)
}

func (c *CircuitBreaker) shouldRecordMetrics(r request.Request) bool {
	_, ok := r.GetUserData(cbreakerRecordMetrics)
	return ok
}

func parseOptions(o Options) (Options, error) {
	if o.FallbackDuration < 0 || o.RecoveryDuration < 0 {
		return o, fmt.Errorf("FallbackDuration and Recoveryuration can not be negative")
	}

	if o.CheckPeriod == 0 {
		o.CheckPeriod = defaultCheckPeriod
	}

	if o.FallbackDuration == 0 {
		o.FallbackDuration = defaultFallbackDuration
	}

	if o.RecoveryDuration == 0 {
		o.RecoveryDuration = defaultRecoveryDuration
	}

	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	return o, nil
}

const (
	cbreakerRecordMetrics = "cbreaker.record"
	cbreakerMetrics       = "cbreaker.metrics"
)
