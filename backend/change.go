package backend

type HostAdded struct {
	Host *Host
}

type HostDeleted struct {
	Name string
}

type LocationAdded struct {
	Host     *Host
	Location *Location
}

type LocationDeleted struct {
	Host       *Host
	LocationId string
}

type LocationUpstreamUpdated struct {
	Host       *Host
	Location   *Location
	UpstreamId string
}

type LocationRateLimitAdded struct {
	Host      *Host
	Location  *Location
	RateLimit *RateLimit
}

type LocationRateLimitDeleted struct {
	Host        *Host
	Location    *Location
	RateLimitId string
}

type LocationRateLimitUpdated struct {
	Host      *Host
	Location  *Location
	RateLimit *RateLimit
}

type UpstreamAdded struct {
	Upstream *Upstream
}

type UpstreamDeleted struct {
	UpstreamId string
}

type EndpointAdded struct {
	Upstream          *Upstream
	Endpoint          *Endpoint
	AffectedLocations []*Location
}

type EndpointDeleted struct {
	Upstream          *Upstream
	EndpointId        string
	AffectedLocations []*Location
}
