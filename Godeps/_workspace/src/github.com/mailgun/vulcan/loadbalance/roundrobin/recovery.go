package roundrobin

type FailureHandler interface {
	// Returns error if something bad happened, returns suggested weights
	AdjustWeights(endpoints []*WeightedEndpoint) ([]SuggestedWeight, error)
	// Resets internal state if any exists
	Reset()
}

type SuggestedWeight interface {
	GetEndpoint() *WeightedEndpoint
	GetWeight() int
}
