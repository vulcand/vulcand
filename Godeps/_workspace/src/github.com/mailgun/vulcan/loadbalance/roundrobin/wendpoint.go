package roundrobin

import (
	"fmt"
	"github.com/mailgun/log"
	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/metrics"
	"net/url"
)

// WeightedEndpoint wraps the endpoint and adds support for weights and failure detection.
type WeightedEndpoint struct {
	// meter accumulates endpoint stats and  for failure detection
	meter metrics.FailRateMeter

	// endpoint is an original endpoint supplied by user
	endpoint endpoint.Endpoint

	// weight holds original weight supplied by user
	weight int

	// effectiveWeight is the weights assigned by the load balancer based on failure
	effectiveWeight int

	// rr is a reference to the parent load balancer
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

func (we *WeightedEndpoint) GetOriginalEndpoint() endpoint.Endpoint {
	return we.endpoint
}

func (we *WeightedEndpoint) GetOriginalWeight() int {
	return we.weight
}

func (we *WeightedEndpoint) GetEffectiveWeight() int {
	return we.effectiveWeight
}

func (we *WeightedEndpoint) GetMeter() metrics.FailRateMeter {
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
