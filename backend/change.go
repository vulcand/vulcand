package backend

type HostAdded struct {
	Host *Host
}

type HostDeleted struct {
	Name        string
	HostEtcdKey string
}

type LocationAdded struct {
	Host     *Host
	Location *Location
}

type LocationDeleted struct {
	Host            *Host
	LocationId      string
	LocationEtcdKey string
}

type LocationUpstreamUpdated struct {
	Host     *Host
	Location *Location
}

type LocationPathUpdated struct {
	Host     *Host
	Location *Location
	Path     string
}

type LocationRateLimitAdded struct {
	Host      *Host
	Location  *Location
	RateLimit *RateLimit
}

type LocationRateLimitDeleted struct {
	Host             *Host
	Location         *Location
	RateLimitId      string
	RateLimitEtcdKey string
}

type LocationRateLimitUpdated struct {
	Host      *Host
	Location  *Location
	RateLimit *RateLimit
}

type LocationConnLimitAdded struct {
	Host      *Host
	Location  *Location
	ConnLimit *ConnLimit
}

type LocationConnLimitDeleted struct {
	Host             *Host
	Location         *Location
	ConnLimitId      string
	ConnLimitEtcdKey string
}

type LocationConnLimitUpdated struct {
	Host      *Host
	Location  *Location
	ConnLimit *ConnLimit
}

type UpstreamAdded struct {
	Upstream *Upstream
}

type UpstreamDeleted struct {
	UpstreamId      string
	UpstreamEtcdKey string
}

type EndpointAdded struct {
	Upstream          *Upstream
	Endpoint          *Endpoint
	AffectedLocations []*Location
}

type EndpointUpdated struct {
	Upstream          *Upstream
	Endpoint          *Endpoint
	AffectedLocations []*Location
}

type EndpointDeleted struct {
	Upstream          *Upstream
	EndpointId        string
	EndpointEtcdKey   string
	AffectedLocations []*Location
}
