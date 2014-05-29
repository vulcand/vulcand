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
	Host     *Host
	Location *Location
}

type LocationPathUpdated struct {
	Host     *Host
	Location *Location
	Path     string
}

type LocationMiddlewareAdded struct {
	Host       *Host
	Location   *Location
	Middleware *MiddlewareInstance
}

type LocationMiddlewareUpdated struct {
	Host       *Host
	Location   *Location
	Middleware *MiddlewareInstance
}

type LocationMiddlewareDeleted struct {
	Host           *Host
	Location       *Location
	MiddlewareId   string
	MiddlewareType string
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

type EndpointUpdated struct {
	Upstream          *Upstream
	Endpoint          *Endpoint
	AffectedLocations []*Location
}

type EndpointDeleted struct {
	Upstream          *Upstream
	EndpointId        string
	AffectedLocations []*Location
}
