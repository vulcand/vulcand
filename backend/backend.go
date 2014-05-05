// Defines interfaces and data structures controlling the proxy configuration and changes
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
	WatchChanges(changes chan interface{}, initialSetup bool) error
}

type StatsGetter interface {
	GetStats(hostname string, locationId string, endpointId string) *EndpointStats
}

type Host struct {
	EtcdKey   string
	Name      string
	Locations []*Location
}

func (l *Host) String() string {
	return fmt.Sprintf("host[name=%s]", l.Name)
}

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

type ConnLimit struct {
	Id          string
	EtcdKey     string `json:",omitempty"`
	Connections int
	Variable    string
}

type RateLimit struct {
	Id            string
	EtcdKey       string `json:",omitempty"`
	PeriodSeconds int
	Burst         int
	Variable      string
	Requests      int
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

type Upstream struct {
	Id        string
	EtcdKey   string `json:",omitempty"`
	Endpoints []*Endpoint
}

func (u *Upstream) String() string {
	return fmt.Sprintf("upstream[id=%s]", u.Id)
}

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

type Change struct {
	Action string
	Parent interface{}
	Child  interface{}
	Keys   map[string]string
	Params map[string]interface{} // refactor the entire thing
}

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
	return nil, fmt.Errorf("Unsupported limiting varuable: '%s'", variable)
}
