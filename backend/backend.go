// Defines interfaces and structures controlling the proxy configuration and changes
package backend

import (
	"fmt"
	"github.com/mailgun/vulcan/limit"
	"strings"
	"time"
)

type Backend interface {
	GetHosts() ([]*Host, error)

	AddHost(name string) error
	DeleteHost(name string) error

	AddLocation(hostname, id, path, upstream string) error
	DeleteLocation(hostname, id string) error
	UpdateLocationUpstream(hostname, id string, upstream string) error

	AddLocationRateLimit(hostname, locationId string, id string, rateLimit *RateLimit) error
	DeleteLocationRateLimit(hostname, locationId, id string) error
	UpdateLocationRateLimit(hostname, locationId string, id string, rateLimit *RateLimit) error

	AddLocationConnLimit(hostname, locationId, id string, connLimit *ConnLimit) error
	DeleteLocationConnLimit(hostname, locationId, id string) error
	UpdateLocationConnLimit(hostname, locationId string, id string, connLimit *ConnLimit) error

	GetUpstreams() ([]*Upstream, error)
	AddUpstream(id string) error
	DeleteUpstream(id string) error
	GetUpstream(id string) (*Upstream, error)

	AddEndpoint(upstreamId, id, url string) error
	DeleteEndpoint(upstreamId, id string) error

	// Watch changes is an entry point for getting the configuration changes
	// as well as the initial configuration. It should be have in the following way:
	// * This should be a blocking function generating events from change.go to the changes channel
	// If the initalSetup is true, it should read the existing configuration and generate the events to the channel
	// just as someone was creating the elements one by one.
	WatchChanges(changes chan interface{}, initialSetup bool) error
}

// Provides realtime stats about endpoint specific to a particular location.
type StatsGetter interface {
	GetStats(hostname string, locationId string, endpointId string) *EndpointStats
}

// Incoming requests are matched by their hostname first.
// Hostname is defined by incoming 'Host' header.
// E.g. curl http://example.com/alice will be matched by the host example.com first.
type Host struct {
	Name      string
	EtcdKey   string
	Locations []*Location
}

func (l *Host) String() string {
	return fmt.Sprintf("host[name=%s]", l.Name)
}

// Hosts contain one or several locations. Each location defines a path - simply a regular expression that will be matched against request's url.
// Location contains link to an upstream and vulcand will use the endpoints from this upstream to serve the request.
// E.g. location loc1 will serve the request curl http://example.com/alice because it matches the path /alice:
type Location struct {
	Hostname   string
	EtcdKey    string `json:",omitempty"`
	Path       string
	Id         string
	Upstream   *Upstream
	ConnLimits []*ConnLimit
	RateLimits []*RateLimit
}

func (l *Location) String() string {
	return fmt.Sprintf("location[id=%s, path=%s]", l.Id, l.Path)
}

// Control simultaneous connections for a location per some variable.
type ConnLimit struct {
	Id          string
	EtcdKey     string `json:",omitempty"`
	Connections int
	Variable    string // Variable defines how the limiting should be done. e.g. 'client.ip' or 'request.header.X-My-Header'
}

// Rate controls how many requests per period of time is allowed for a location.
// Existing implementation is based on the token bucket algorightm http://en.wikipedia.org/wiki/Token_bucket
type RateLimit struct {
	Id            string
	EtcdKey       string `json:",omitempty"`
	PeriodSeconds int    // Period in seconds, e.g. 3600 to set up hourly rates
	Burst         int    // Burst count, allowes some extra variance for requests exceeding the average rate
	Variable      string // Variable defines how the limiting should be done. e.g. 'client.ip' or 'request.header.X-My-Header'
	Requests      int    // Allowed average requests
}

func NewRateLimit(requests int, variable string, burst int, periodSeconds int) (*RateLimit, error) {
	if _, err := VariableToMapper(variable); err != nil {
		return nil, err
	}
	if requests <= 0 {
		return nil, fmt.Errorf("Requests should be > 0, got %d", requests)
	}
	if burst < 0 {
		return nil, fmt.Errorf("Burst should be >= 0, got %d", burst)
	}
	if periodSeconds <= 0 {
		return nil, fmt.Errorf("Period seconds should be > 0, got %d", periodSeconds)
	}
	return &RateLimit{
		Requests:      requests,
		Variable:      variable,
		Burst:         burst,
		PeriodSeconds: periodSeconds,
	}, nil
}

func (rl *RateLimit) String() string {
	return fmt.Sprintf("ratelimit[id=%s, var=%s, reqs/%s=%d, burst=%d]", rl.Id, rl.Variable, time.Duration(rl.PeriodSeconds)*time.Second, rl.Requests, rl.Burst)
}

func NewConnLimit(connections int, variable string) (*ConnLimit, error) {
	if _, err := VariableToMapper(variable); err != nil {
		return nil, err
	}
	if connections < 0 {
		return nil, fmt.Errorf("Connections should be > 0, got %d", connections)
	}
	return &ConnLimit{
		Connections: connections,
		Variable:    variable,
	}, nil
}

func (cl *ConnLimit) String() string {
	return fmt.Sprintf("connlimit[id=%s, conn=%d, var=%s]", cl.Id, cl.Connections, cl.Variable)
}

// Upstream is a collection of endpoints. Each location is assigned an upstream. Changing assigned upstream
// of the location gracefully redirects the traffic to the new endpoints of the upstream.
type Upstream struct {
	Id        string
	EtcdKey   string `json:",omitempty"`
	Endpoints []*Endpoint
}

func (u *Upstream) String() string {
	return fmt.Sprintf("upstream[id=%s]", u.Id)
}

// Endpoint is a final destination of the request
type Endpoint struct {
	EtcdKey string `json:",omitempty"`
	Id      string
	Url     string
	Stats   *EndpointStats
}

func (e *Endpoint) String() string {
	if e.Stats == nil {
		return fmt.Sprintf("endpoint[id=%s, url=%s]", e.Id, e.Url)
	} else {
		return fmt.Sprintf("endpoint[id=%s, url=%s, %s]", e.Id, e.Url, e.Stats)
	}
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

// Converts varaiable string to a mapper function used in rate limiters
func VariableToMapper(variable string) (limit.MapperFn, error) {
	if variable == "client.ip" {
		return limit.MapClientIp, nil
	}
	if variable == "request.host" {
		return limit.MapRequestHost, nil
	}
	if strings.HasPrefix(variable, "request.header.") {
		header := strings.TrimPrefix(variable, "request.header.")
		if len(header) == 0 {
			return nil, fmt.Errorf("Wrong header: %s", header)
		}
		return limit.MakeMapRequestHeader(header), nil
	}
	return nil, fmt.Errorf("Unsupported limiting variable: '%s'", variable)
}
