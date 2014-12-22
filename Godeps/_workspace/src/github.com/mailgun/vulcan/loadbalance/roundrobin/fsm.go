package roundrobin

import (
	"fmt"
	"time"

	"github.com/mailgun/timetools"
	"github.com/mailgun/vulcan/metrics"
)

// This handler increases weights on endpoints that perform better than others
// it also rolls back to original weights if the endpoints have changed.
type FSMHandler struct {
	// As usual, control time in tests
	timeProvider timetools.TimeProvider
	// Time that freezes state machine to accumulate stats after updating the weights
	backoffDuration time.Duration
	// Timer is set to give probing some time to take place
	timer time.Time
	// Endpoints for this round
	endpoints []*WeightedEndpoint
	// Precalculated original weights
	originalWeights []SuggestedWeight
	// Last returned weights
	lastWeights []SuggestedWeight
}

const (
	// This is the maximum weight that handler will set for the endpoint
	FSMMaxWeight = 4096
	// Multiplier for the endpoint weight
	FSMGrowFactor = 16
)

func NewFSMHandler() (*FSMHandler, error) {
	return NewFSMHandlerWithOptions(&timetools.RealTime{})
}

func NewFSMHandlerWithOptions(timeProvider timetools.TimeProvider) (*FSMHandler, error) {
	if timeProvider == nil {
		return nil, fmt.Errorf("time provider can not be nil")
	}
	return &FSMHandler{
		timeProvider: timeProvider,
	}, nil
}

func (fsm *FSMHandler) Init(endpoints []*WeightedEndpoint) {
	fsm.originalWeights = makeOriginalWeights(endpoints)
	fsm.lastWeights = fsm.originalWeights
	fsm.endpoints = endpoints
	if len(endpoints) > 0 {
		fsm.backoffDuration = endpoints[0].meter.GetWindowSize() / 2
	}
	fsm.timer = fsm.timeProvider.UtcNow().Add(-1 * time.Second)
}

// Called on every load balancer NextEndpoint call, returns the suggested weights
// on every call, can adjust weights if needed.
func (fsm *FSMHandler) AdjustWeights() ([]SuggestedWeight, error) {
	// In this case adjusting weights would have no effect, so do nothing
	if len(fsm.endpoints) < 2 {
		return fsm.originalWeights, nil
	}
	// Metrics are not ready
	if !metricsReady(fsm.endpoints) {
		return fsm.originalWeights, nil
	}
	if !fsm.timerExpired() {
		return fsm.lastWeights, nil
	}
	// Select endpoints with highest error rates and lower their weight
	good, bad := splitEndpoints(fsm.endpoints)
	// No endpoints that are different by their quality, so converge weights
	if len(bad) == 0 || len(good) == 0 {
		weights, changed := fsm.convergeWeights()
		if changed {
			fsm.lastWeights = weights
			fsm.setTimer()
		}
		return fsm.lastWeights, nil
	}
	fsm.lastWeights = fsm.adjustWeights(good, bad)
	fsm.setTimer()
	return fsm.lastWeights, nil
}

func (fsm *FSMHandler) convergeWeights() ([]SuggestedWeight, bool) {
	weights := make([]SuggestedWeight, len(fsm.endpoints))
	// If we have previoulsy changed endpoints try to restore weights to the original state
	changed := false
	for i, e := range fsm.endpoints {
		weights[i] = &EndpointWeight{e, decrease(e.GetOriginalWeight(), e.GetEffectiveWeight())}
		if e.GetEffectiveWeight() != e.GetOriginalWeight() {
			changed = true
		}
	}
	return normalizeWeights(weights), changed
}

func (fsm *FSMHandler) adjustWeights(good map[string]bool, bad map[string]bool) []SuggestedWeight {
	// Increase weight on good endpoints
	weights := make([]SuggestedWeight, len(fsm.endpoints))
	for i, e := range fsm.endpoints {
		if good[e.GetId()] && increase(e.GetEffectiveWeight()) <= FSMMaxWeight {
			weights[i] = &EndpointWeight{e, increase(e.GetEffectiveWeight())}
		} else {
			weights[i] = &EndpointWeight{e, e.GetEffectiveWeight()}
		}
	}
	return normalizeWeights(weights)
}

func weightsGcd(weights []SuggestedWeight) int {
	divisor := -1
	for _, w := range weights {
		if divisor == -1 {
			divisor = w.GetWeight()
		} else {
			divisor = gcd(divisor, w.GetWeight())
		}
	}
	return divisor
}

func normalizeWeights(weights []SuggestedWeight) []SuggestedWeight {
	gcd := weightsGcd(weights)
	if gcd <= 1 {
		return weights
	}
	for _, w := range weights {
		w.SetWeight(w.GetWeight() / gcd)
	}
	return weights
}

func (fsm *FSMHandler) setTimer() {
	fsm.timer = fsm.timeProvider.UtcNow().Add(fsm.backoffDuration)
}

func (fsm *FSMHandler) timerExpired() bool {
	return fsm.timer.Before(fsm.timeProvider.UtcNow())
}

func metricsReady(endpoints []*WeightedEndpoint) bool {
	for _, e := range endpoints {
		if !e.meter.IsReady() {
			return false
		}
	}
	return true
}

func increase(weight int) int {
	return weight * FSMGrowFactor
}

func decrease(target, current int) int {
	adjusted := current / FSMGrowFactor
	if adjusted < target {
		return target
	} else {
		return adjusted
	}
}

func makeOriginalWeights(endpoints []*WeightedEndpoint) []SuggestedWeight {
	weights := make([]SuggestedWeight, len(endpoints))
	for i, e := range endpoints {
		weights[i] = &EndpointWeight{
			Weight:   e.GetOriginalWeight(),
			Endpoint: e,
		}
	}
	return weights
}

// splitEndpoints splits endpoints into two groups of endpoints with bad and good failure rate.
// It does compare relative performances of the endpoints though, so if all endpoints have approximately the same error rate
// this function returns the result as if all endpoints are equally good.
func splitEndpoints(endpoints []*WeightedEndpoint) (map[string]bool, map[string]bool) {

	failRates := make([]float64, len(endpoints))

	for i, e := range endpoints {
		failRates[i] = e.failRate()
	}

	g, b := metrics.SplitFloat64(1.5, 0, failRates)
	good, bad := make(map[string]bool, len(g)), make(map[string]bool, len(b))

	for _, e := range endpoints {
		if g[e.failRate()] {
			good[e.GetId()] = true
		} else {
			bad[e.GetId()] = true
		}
	}

	return good, bad
}
