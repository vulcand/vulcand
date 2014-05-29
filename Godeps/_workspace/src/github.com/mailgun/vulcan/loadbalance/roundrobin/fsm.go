package roundrobin

import (
	"fmt"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	"math"
	"time"
)

// This handler increases weights on endpoints that perform better than others
// it also rolls back to original weights if the endpoints have changed.
type FSMHandler struct {
	// As usual, control time in tests
	timeProvider timetools.TimeProvider
	// Time that freezes state machine to accumulate stats after updating the weights
	backoffDuration time.Duration
	// Current state of the state machine
	state FSMState
	// Timer is set to give probing some time to take place
	timer time.Time
	// Probing changes endpoint weights and remembers the weight so it can go back in case of failure
	probedEndpoints []*changedEndpoint
}

type FSMState int

const (
	// Initial state of the FSM
	FSMStart = iota
	// FSM has increased weights and accumulates stats
	FSMProbing = iota
	// FSM rolls back the weights to original ones after unsucessful adjustment
	FSMRollback = iota
	// Stat machine is getting back to the original state
	FSMRevert = iota
)

const (
	// This is the maximum weight that handler will set for the endpoint
	FSMMaxWeight = 4096
	// Multiplier for the endpoint weight
	FSMGrowFactor = 8
	// This is how long handler after any action taken on the weights
	FSMDefaultProbingPeriod = 4 * time.Second
)

func NewFSMHandler() (*FSMHandler, error) {
	return NewFSMHandlerWithOptions(&timetools.RealTime{}, FSMDefaultProbingPeriod)
}

func NewFSMHandlerWithOptions(timeProvider timetools.TimeProvider, duration time.Duration) (*FSMHandler, error) {
	if timeProvider == nil {
		return nil, fmt.Errorf("time provider can not be nil")
	}
	if duration < time.Second {
		return nil, fmt.Errorf("supply some backoff duration >= time.Second")
	}
	return &FSMHandler{
		timeProvider:    timeProvider,
		backoffDuration: duration,
	}, nil
}

// Called on every load balancer NextEndpoint call. In case if there's nothing to do, returns nil, nil
func (fsm *FSMHandler) AdjustWeights(endpoints []*WeightedEndpoint) ([]SuggestedWeight, error) {
	// In this case adjusting weights would have no effect, so do nothing
	if len(endpoints) < 2 {
		return nil, nil
	}
	switch fsm.state {
	case FSMStart:
		return fsm.onStart(endpoints)
	case FSMProbing:
		return fsm.onProbing(endpoints)
	case FSMRollback:
		return fsm.onRollback(endpoints)
	case FSMRevert:
		return fsm.onRollback(endpoints)
	}
	return nil, fmt.Errorf("Unsupported state")
}

func (fsm *FSMHandler) onStart(endpoints []*WeightedEndpoint) ([]SuggestedWeight, error) {
	w := &WeightWatcher{fsm: fsm}
	failRate := avgFailRate(endpoints)
	// No errors, so let's see if we can recover weights of previosly changed endpoints to the original state
	if failRate == 0 {
		// If we have previoulsy changed endpoints try to restore weights to the original state
		for _, e := range endpoints {
			if e.GetEffectiveWeight() != e.GetOriginalWeight() {
				// Adjust effective weight back to the original weight in stages
				w.setWeight(e, decrease(e.GetOriginalWeight(), e.GetEffectiveWeight()))
			}
		}
		weights := w.getWeights()
		// We have just restored the weights, go to revert state.
		if len(weights) != 0 {
			fsm.setTimer()
			fsm.state = FSMRevert
		}
		return weights, nil
	} else {
		log.Infof("%s reports average fail rate %f", fsm, failRate)
		if !metricsReady(endpoints) {
			log.Infof("%s skip cycle, metrics are not ready yet", fsm)
			return nil, nil
		}
		// Select endpoints with highest error rates and lower their weight
		good, bad := splitEndpoints(endpoints)
		log.Infof("%s better endpoints: %s", fsm, good)
		log.Infof("%s worse endpoints: %s", fsm, bad)
		// No endpoints that are different by their quality
		if len(bad) == 0 || len(good) == 0 {
			log.Infof("%s all endpoints have roughly same error rate", fsm)
			return nil, nil
		}
		// Increase weight on good endpoints
		for _, e := range good {
			if increase(e.GetEffectiveWeight()) <= FSMMaxWeight {
				w.setWeight(e, increase(e.GetEffectiveWeight()))
			}
		}
		weights := w.getWeights()
		if len(weights) != 0 {
			fsm.state = FSMProbing
			fsm.probedEndpoints = w.getChangedEndpoints()
			fsm.setTimer()
		}
		return weights, nil
	}
}

func (fsm *FSMHandler) onProbing(endpoints []*WeightedEndpoint) ([]SuggestedWeight, error) {
	if !fsm.timerExpired() {
		return nil, nil
	}

	// Now revise the good endpoints and see if we made situation worse
	w := &WeightWatcher{fsm: fsm}
	for _, e := range fsm.probedEndpoints {
		if greater(e.endpoint.failRateMeter.GetRate(), e.failRatioBefore) {
			// Oops, we made it worse, revert the weights back and go to rollback state
			for _, e := range fsm.probedEndpoints {
				w.setWeight(e.endpoint, e.weightBefore)
			}
		}
	}

	weights := w.getWeights()
	// This means that we've just reversed the rates
	if len(weights) != 0 {
		fsm.probedEndpoints = nil
		fsm.state = FSMRollback
		fsm.setTimer()
		return weights, nil
	}

	// We have not made the situation worse, so go back to the starting point and continue the cycle
	log.Infof("%s Probing new rates was successfull, COMMITING the new rates", fsm)
	fsm.state = FSMStart
	return nil, nil
}

func (fsm *FSMHandler) onRollback(endpoints []*WeightedEndpoint) ([]SuggestedWeight, error) {
	if !fsm.timerExpired() {
		return nil, nil
	}
	log.Infof("%s timer expired", fsm)
	fsm.state = FSMStart
	return nil, nil
}

func (fsm *FSMHandler) setTimer() {
	fsm.timer = fsm.timeProvider.UtcNow().Add(fsm.backoffDuration)
}

func (fsm *FSMHandler) timerExpired() bool {
	return fsm.timer.Before(fsm.timeProvider.UtcNow())
}

func (fsm *FSMHandler) GetState() FSMState {
	return fsm.state
}

func (fsm *FSMHandler) Reset() {
	fsm.state = FSMStart
	fsm.timer = fsm.timeProvider.UtcNow().Add(-1 * time.Second)
	fsm.probedEndpoints = nil
}

func (fsm *FSMHandler) String() string {
	return fmt.Sprintf("FSM(state=%s)", stateToString(fsm.state))
}

// Splits endpoint into two groups of endpoints with bad performance and good performance. It does compare relative
// performances of the endpoints though, so if all endpoints have the same performance,
func splitEndpoints(endpoints []*WeightedEndpoint) (good []*WeightedEndpoint, bad []*WeightedEndpoint) {
	avg := avgFailRate(endpoints)
	for _, e := range endpoints {
		if greater(e.failRateMeter.GetRate(), avg) {
			bad = append(bad, e)
		} else {
			good = append(good, e)
		}
	}
	return good, bad
}

func metricsReady(endpoints []*WeightedEndpoint) bool {
	for _, e := range endpoints {
		if !e.failRateMeter.IsReady() {
			return false
		}
	}
	return true
}

// Compare two fail rates by neglecting the insignificant differences
func greater(a, b float64) bool {
	return math.Floor(a*10) > math.Ceil(b*10)
}

func avgFailRate(endpoints []*WeightedEndpoint) float64 {
	r := float64(0)
	for _, e := range endpoints {
		eRate := e.failRateMeter.GetRate()
		r += eRate
	}
	return r / float64(len(endpoints))
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

func stateToString(state FSMState) string {
	switch state {
	case FSMStart:
		return "START"
	case FSMProbing:
		return "PROBING"
	case FSMRollback:
		return "ROLLBACK"
	case FSMRevert:
		return "REVERT"
	}
	return "UNKNOWN"
}

type WeightWatcher struct {
	weights map[string]*changedEndpoint
	fsm     *FSMHandler
}

func (w *WeightWatcher) setWeight(we *WeightedEndpoint, weight int) {
	if w.weights == nil {
		w.weights = make(map[string]*changedEndpoint)
	}
	log.Infof("%s proposing weight of %s to %d", w.fsm, we, weight)
	w.weights[we.GetId()] = &changedEndpoint{
		newWeight:       weight,
		weightBefore:    we.GetEffectiveWeight(),
		failRatioBefore: we.failRateMeter.GetRate(),
		endpoint:        we,
	}
}

func (w *WeightWatcher) getWeights() []SuggestedWeight {
	if len(w.weights) == 0 {
		return nil
	}
	out := make([]SuggestedWeight, len(w.weights))
	i := 0
	for _, w := range w.weights {
		out[i] = w
		i += 1
	}
	return out
}

func (w *WeightWatcher) getChangedEndpoints() []*changedEndpoint {
	if len(w.weights) == 0 {
		return nil
	}
	out := make([]*changedEndpoint, len(w.weights))
	i := 0
	for _, w := range w.weights {
		out[i] = w
		i += 1
	}
	return out
}

type changedEndpoint struct {
	failRatioBefore float64
	endpoint        *WeightedEndpoint
	weightBefore    int
	newWeight       int
}

func (ce *changedEndpoint) GetEndpoint() *WeightedEndpoint {
	return ce.endpoint
}

func (ce *changedEndpoint) GetWeight() int {
	return ce.newWeight
}
