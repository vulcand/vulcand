// Defines interfaces and structures controlling the proxy configuration and changes
package backend

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcand/plugin"
	"regexp"
)

type Backend interface {
	GetHosts() ([]*Host, error)
	AddHost(*Host) (*Host, error)
	GetHost(name string) (*Host, error)
	DeleteHost(name string) error

	AddLocation(*Location) (*Location, error)
	GetLocation(hostname, id string) (*Location, error)
	UpdateLocationUpstream(hostname, id string, upstream string) (*Location, error)
	DeleteLocation(hostname, id string) error

	AddLocationMiddleware(hostname, locationId string, m *MiddlewareInstance) (*MiddlewareInstance, error)
	GetLocationMiddleware(hostname, locationId string, mType, id string) (*MiddlewareInstance, error)
	UpdateLocationMiddleware(hostname, locationId string, m *MiddlewareInstance) (*MiddlewareInstance, error)
	DeleteLocationMiddleware(hostname, locationId, mType, id string) error

	GetUpstreams() ([]*Upstream, error)
	AddUpstream(*Upstream) (*Upstream, error)
	GetUpstream(id string) (*Upstream, error)
	DeleteUpstream(id string) error

	AddEndpoint(*Endpoint) (*Endpoint, error)
	GetEndpoint(upstreamId, id string) (*Endpoint, error)
	DeleteEndpoint(upstreamId, id string) error

	// Watch changes is an entry point for getting the configuration changes
	// as well as the initial configuration. It should be have in the following way:
	// * This should be a blocking function generating events from change.go to the changes channel
	// If the initalSetup is true, it should read the existing configuration and generate the events to the channel
	// just as someone was creating the elements one by one.
	WatchChanges(changes chan interface{}, initialSetup bool) error

	// Returns registry with the supported plugins
	GetRegistry() *Registry
}

// Provides realtime stats about endpoint specific to a particular location.
type StatsGetter interface {
	GetStats(hostname string, locationId string, e *Endpoint) *EndpointStats
}

// Incoming requests are matched by their hostname first. Hostname is defined by incoming 'Host' header.
// E.g. curl http://example.com/alice will be matched by the host example.com first.
type Host struct {
	Name      string
	Locations []*Location
}

func NewHost(name string) (*Host, error) {
	if name == "" {
		return nil, fmt.Errorf("Hostname can not be empty")
	}
	return &Host{
		Name:      name,
		Locations: []*Location{},
	}, nil
}

func (h *Host) String() string {
	return fmt.Sprintf("Host(name=%s, locations=%s)", h.Name, h.Locations)
}

func (h *Host) GetId() string {
	return h.Name
}

// Hosts contain one or several locations. Each location defines a path - simply a regular expression that will be matched against request's url.
// Location contains link to an upstream and vulcand will use the endpoints from this upstream to serve the request.
// E.g. location loc1 will serve the request curl http://example.com/alice because it matches the path /alice:
type Location struct {
	Hostname    string
	Path        string
	Id          string
	Upstream    *Upstream
	Middlewares []*MiddlewareInstance
}

// Wrapper that contains information about this middleware backend-specific data used for serialization/deserialization
type MiddlewareInstance struct {
	Id         string
	Priority   int
	Type       string
	Middleware Middleware
}

func NewLocation(hostname, id, path, upstreamId string) (*Location, error) {
	if len(path) == 0 || len(hostname) == 0 || len(upstreamId) == 0 {
		return nil, fmt.Errorf("Supply valid hostname, path and upstream id")
	}

	// Make sure location path is a valid regular expression
	if _, err := regexp.Compile(path); err != nil {
		return nil, fmt.Errorf("Path should be a valid Golang regular expression")
	}

	return &Location{
		Hostname:    hostname,
		Path:        path,
		Id:          id,
		Upstream:    &Upstream{Id: upstreamId, Endpoints: []*Endpoint{}},
		Middlewares: []*MiddlewareInstance{},
	}, nil
}

func (l *Location) String() string {
	return fmt.Sprintf(
		"Location(hostname=%s, id=%s, path=%s, upstream=%s, middlewares=%s)",
		l.Hostname, l.Id, l.Path, l.Upstream, l.Middlewares)
}

func (l *Location) GetId() string {
	return l.Id
}

// Upstream is a collection of endpoints. Each location is assigned an upstream. Changing assigned upstream
// of the location gracefully redirects the traffic to the new endpoints of the upstream.
type Upstream struct {
	Id        string
	Endpoints []*Endpoint
}

func NewUpstream(id string) (*Upstream, error) {
	return &Upstream{
		Id:        id,
		Endpoints: []*Endpoint{},
	}, nil
}

func (u *Upstream) String() string {
	return fmt.Sprintf("Upstream(id=%s, endpoints=%s)", u.Id, u.Endpoints)
}

func (u *Upstream) GetId() string {
	return u.Id
}

// Endpoint is a final destination of the request
type Endpoint struct {
	Id         string
	Url        string
	UpstreamId string
	Stats      *EndpointStats
}

func NewEndpoint(upstreamId, id, url string) (*Endpoint, error) {
	if upstreamId == "" {
		return nil, fmt.Errorf("Upstream id '%s' can not be empty")
	}
	if _, err := netutils.ParseUrl(url); err != nil {
		return nil, fmt.Errorf("Endpoint url '%s' is not valid", url)
	}
	return &Endpoint{
		UpstreamId: upstreamId,
		Id:         id,
		Url:        url,
	}, nil
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("Endpoint(id=%s, up=%s, url=%s, stats=%s)", e.Id, e.UpstreamId, e.Url, e.Stats)
}

func (e *Endpoint) GetId() string {
	return e.Id
}

func (e *Endpoint) GetUniqueId() string {
	return fmt.Sprintf("%s.%s", e.UpstreamId, e.Id)
}

// Endpoint's realtime stats about endpoint
type EndpointStats struct {
	Successes     int64
	Failures      int64
	FailRate      float64
	PeriodSeconds int
}

func (e *EndpointStats) String() string {
	reqsSec := (e.Failures + e.Successes) / int64(e.PeriodSeconds)
	return fmt.Sprintf("%d requests/sec, %.2f failures/sec", reqsSec, e.FailRate)
}

type NotFoundError struct {
	Message string
}

func (n *NotFoundError) Error() string {
	if n.Message != "" {
		return n.Message
	} else {
		return "Object not found"
	}
}

type AlreadyExistsError struct {
	Message string
}

func (n *AlreadyExistsError) Error() string {
	return "Object already exists"
}
