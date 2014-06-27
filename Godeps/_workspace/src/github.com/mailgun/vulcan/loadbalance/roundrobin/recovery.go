package roundrobin

type FailureHandler interface {
	// Returns error if something bad happened, returns suggested weights
	AdjustWeights() ([]SuggestedWeight, error)
	// Initializes handler with current set of endpoints. Will be called
	// each time endpoints are added or removed from the load balancer
	// to give failure handler a chance to set it's itenral state
	Init(endpoints []*WeightedEndpoint)
}

type SuggestedWeight interface {
	GetEndpoint() *WeightedEndpoint
	GetWeight() int
	SetWeight(int)
}

type EndpointWeight struct {
	Endpoint *WeightedEndpoint
	Weight   int
}

func (ew *EndpointWeight) GetEndpoint() *WeightedEndpoint {
	return ew.Endpoint
}

func (ew *EndpointWeight) GetWeight() int {
	return ew.Weight
}

func (ew *EndpointWeight) SetWeight(w int) {
	ew.Weight = w
}
