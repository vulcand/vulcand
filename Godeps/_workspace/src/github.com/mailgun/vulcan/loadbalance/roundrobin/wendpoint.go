package roundrobin

import (
	"fmt"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	"net/url"
)

// Wraps the endpoint and adds support for weights and failure detection
type WeightedEndpoint struct {
	// This meter will accumulate endpoint stats in realtime and can be used
	// for failure detection in the failure handlers.
	meter FailRateMeter
	// Original endpoint supplied by user
	endpoint Endpoint
	// Original weight supplied by user
	weight int
	// Current weight that is in effect at the moment
	effectiveWeight int
	// Reference to the parent load balancer
	rr *RoundRobin
}

func (we *WeightedEndpoint) String() string {
	return fmt.Sprintf("WeightedEndpoint(id=%s, url=%s, weight=%d, effectiveWeight=%d, failRate=%f)",
		we.GetId(), we.GetUrl(), we.weight, we.effectiveWeight, we.meter.GetRate())
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
	return we.meter
}

func (we *WeightedEndpoint) failRate() float64 {
	return we.meter.GetRate()
}

type WeightedEndpoints []*WeightedEndpoint

func (we WeightedEndpoints) Len() int {
	return len(we)
}

func (we WeightedEndpoints) Swap(i, j int) {
	we[i], we[j] = we[j], we[i]
}

func (we WeightedEndpoints) Less(i, j int) bool {
	return we[i].meter.GetRate() < we[j].meter.GetRate()
}
