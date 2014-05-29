// Dynamic weighted round robin load balancer
package roundrobin

import (
	"fmt"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Dynamic weighted round robin load balancer.
type RoundRobin struct {
	mutex *sync.Mutex
	// Current index (starts from -1)
	index         int
	endpoints     []*WeightedEndpoint
	currentWeight int
	options       Options
}

type Options struct {
	// Control time in tests
	TimeProvider timetools.TimeProvider
	// Algorithm that reacts on the failures and can adjust weights
	FailureHandler FailureHandler
}

// Set additional parameters for the endpoint can be supplied when adding endpoint
type EndpointOptions struct {
	// Relative weight for the enpoint to other enpoints in the load balancer
	Weight int
	// Meter that will be used to detect failures
	Meter FailRateMeter
}

// Wraps the endpoint and adds support for weights and failure detection
type WeightedEndpoint struct {
	// This meter will accumulate endpoint stats in realtime and can be used
	// for failure detection in the failure handlers.
	failRateMeter FailRateMeter
	// Original endpoint supplied by user
	endpoint Endpoint
	// Original weight supplied by user
	weight int
	// Current weight that is in effect at the moment
	effectiveWeight int
	// Reference to the parent load balancer
	rr *RoundRobin
}

func NewRoundRobin() (*RoundRobin, error) {
	return NewRoundRobinWithOptions(Options{})
}

func NewRoundRobinWithOptions(o Options) (*RoundRobin, error) {
	o, err := validateOptions(o)
	if err != nil {
		return nil, err
	}
	rr := &RoundRobin{
		options:   o,
		index:     -1,
		mutex:     &sync.Mutex{},
		endpoints: []*WeightedEndpoint{},
	}
	return rr, nil
}

func (r *RoundRobin) NextEndpoint(req Request) (Endpoint, error) {
	e, err := r.nextEndpoint(req)
	if err != nil {
		return e, err
	}
	lastAttempt := req.GetLastAttempt()
	// This is the first try, so just return the selected endpoint
	if lastAttempt == nil {
		return e, err
	}
	// Try to prevent failover to the same endpoint that we've seen before,
	// that reduces the probability of the scenario when failover hits same endpoint
	// on the next attempt and fails, so users will see a failed request.
	if lastAttempt.GetEndpoint().GetId() == e.GetId() {
		log.Infof("Preventing failover to the same endpoint, whoa")
		return r.nextEndpoint(req)
	}
	return e, err
}

func (r *RoundRobin) nextEndpoint(req Request) (Endpoint, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(r.endpoints) == 0 {
		return nil, fmt.Errorf("No endpoints")
	}

	// Adjust weights based on endpoints failure rates
	r.adjustWeights()

	// The algo below may look messy, but is actually very simple
	// it calculates the GCD  and subtracts it on every iteration, what interleaves endpoints
	// and allows us not to build an iterator every time we readjust weights

	// GCD across all enabled endpoints
	gcd := r.weightGcd()
	// Maximum weight across all enabled endpoints
	max := r.maxWeight()

	for {
		r.index = (r.index + 1) % len(r.endpoints)
		if r.index == 0 {
			r.currentWeight = r.currentWeight - gcd
			if r.currentWeight <= 0 {
				r.currentWeight = max
				if r.currentWeight == 0 {
					return nil, fmt.Errorf("All endpoints have 0 weight")
				}
			}
		}
		e := r.endpoints[r.index]
		if e.effectiveWeight >= r.currentWeight {
			return e.endpoint, nil
		}
	}

	// We did full circle and found no available endpoints
	return nil, fmt.Errorf("No available endpoints!")
}

func (r *RoundRobin) adjustWeights() {
	if r.options.FailureHandler == nil {
		return
	}
	weights, err := r.options.FailureHandler.AdjustWeights(r.endpoints)
	if err != nil {
		log.Errorf("%s returned error: %s", r.options.FailureHandler, err)
		return
	}
	if len(weights) == 0 {
		return
	}
	for _, w := range weights {
		w.GetEndpoint().setEffectiveWeight(w.GetWeight())
	}
	r.resetIterator()
}

func (r *RoundRobin) GetEndpoints() []*WeightedEndpoint {
	return r.endpoints
}

func (rr *RoundRobin) AddEndpoint(endpoint Endpoint) error {
	return rr.AddEndpointWithOptions(endpoint, EndpointOptions{})
}

// In case if endpoint is already present in the load balancer, returns error
func (r *RoundRobin) AddEndpointWithOptions(endpoint Endpoint, options EndpointOptions) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if endpoint == nil {
		return fmt.Errorf("Endpoint can't be nil")
	}

	if e, _ := r.findEndpointByUrl(endpoint.GetUrl()); e != nil {
		return fmt.Errorf("Endpoint already exists")
	}

	we, err := r.newWeightedEndpoint(endpoint, options)
	if err != nil {
		return err
	}

	r.endpoints = append(r.endpoints, we)
	r.resetState()
	return nil
}

func (r *RoundRobin) resetIterator() {
	r.index = -1
	r.currentWeight = 0
}

func (r *RoundRobin) resetState() {
	r.resetIterator()
	if r.options.FailureHandler != nil {
		r.options.FailureHandler.Reset()
	}
}

func (r *RoundRobin) findEndpointByUrl(iu *url.URL) (*WeightedEndpoint, int) {
	if len(r.endpoints) == 0 {
		return nil, -1
	}
	for i, e := range r.endpoints {
		u := e.GetUrl()
		if u.Path == iu.Path && u.Host == iu.Host && u.Scheme == iu.Scheme {
			return e, i
		}
	}
	return nil, -1
}

func (r *RoundRobin) FindEndpointByUrl(url string) *WeightedEndpoint {
	out, err := netutils.ParseUrl(url)
	if err != nil {
		return nil
	}
	found, _ := r.findEndpointByUrl(out)
	return found
}

func (r *RoundRobin) FindEndpointById(id string) *WeightedEndpoint {
	if len(r.endpoints) == 0 {
		return nil
	}
	for _, e := range r.endpoints {
		if e.GetId() == id {
			return e
		}
	}
	return nil
}

func (rr *RoundRobin) newWeightedEndpoint(endpoint Endpoint, options EndpointOptions) (*WeightedEndpoint, error) {
	// Treat weight 0 as a default value passed by customer
	if options.Weight == 0 {
		options.Weight = 1
	}
	if options.Weight < 0 {
		return nil, fmt.Errorf("Weight should be >=0")
	}

	if options.Meter == nil {
		meter, err := NewRollingMeter(
			endpoint, 10, time.Second, rr.options.TimeProvider, IsNetworkError)
		if err != nil {
			return nil, err
		}
		options.Meter = meter
	}

	return &WeightedEndpoint{
		failRateMeter:   options.Meter,
		endpoint:        endpoint,
		weight:          options.Weight,
		effectiveWeight: options.Weight,
		rr:              rr,
	}, nil
}

func (r *RoundRobin) RemoveEndpoint(endpoint Endpoint) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	e, index := r.findEndpointByUrl(endpoint.GetUrl())
	if e == nil {
		return fmt.Errorf("Endpoint not found")
	}
	r.endpoints = append(r.endpoints[:index], r.endpoints[index+1:]...)
	r.resetState()
	return nil
}

func (rr *RoundRobin) ProcessRequest(Request) (*http.Response, error) {
	return nil, nil
}

func (rr *RoundRobin) ProcessResponse(req Request, a Attempt) {
}

func (rr *RoundRobin) ObserveRequest(Request) {
}

func (rr *RoundRobin) ObserveResponse(req Request, a Attempt) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	if a == nil || a.GetEndpoint() == nil {
		return
	}
	we, _ := rr.findEndpointByUrl(a.GetEndpoint().GetUrl())
	if we == nil {
		return
	}
	// Update stats for the endpoint after the request was done
	we.failRateMeter.ObserveResponse(req, a)
}

func (rr *RoundRobin) maxWeight() int {
	max := -1
	for _, e := range rr.endpoints {
		if e.effectiveWeight > max {
			max = e.effectiveWeight
		}
	}
	return max
}

func (rr *RoundRobin) weightGcd() int {
	divisor := -1
	for _, e := range rr.endpoints {
		if divisor == -1 {
			divisor = e.effectiveWeight
		} else {
			divisor = gcd(divisor, e.effectiveWeight)
		}
	}
	return divisor
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func (we *WeightedEndpoint) String() string {
	return fmt.Sprintf("WeightedEndpoint(id=%s, url=%s, weight=%d, effectiveWeight=%d, failRate=%f)",
		we.GetId(), we.GetUrl(), we.weight, we.effectiveWeight, we.failRateMeter.GetRate())
}

func (we *WeightedEndpoint) GetId() string {
	return we.endpoint.GetId()
}

func (we *WeightedEndpoint) GetUrl() *url.URL {
	return we.endpoint.GetUrl()
}

func (we *WeightedEndpoint) setEffectiveWeight(w int) {
	log.Infof("%s setting effective weight to: %d", we, w)
	we.effectiveWeight = w
}

func (we *WeightedEndpoint) GetOriginalWeight() int {
	return we.weight
}

func (we *WeightedEndpoint) GetEffectiveWeight() int {
	return we.effectiveWeight
}

func (we *WeightedEndpoint) GetMeter() FailRateMeter {
	return we.failRateMeter
}

func validateOptions(o Options) (Options, error) {
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}

	if o.FailureHandler == nil {
		failureHandler, err := NewFSMHandler()
		if err != nil {
			return o, err
		}
		o.FailureHandler = failureHandler
	}
	return o, nil
}
