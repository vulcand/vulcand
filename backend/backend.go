package backend

import (
	"fmt"
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

	AddLocationConnLimit(hostname, locationId, id string, connLimit *ConnLimit) error
	DeleteLocationConnLimit(hostname, locationId, id string) error

	GetUpstreams() ([]*Upstream, error)
	AddUpstream(id string) error
	DeleteUpstream(id string) error

	AddEndpoint(upstreamId, id, url string) error
	DeleteEndpoint(upstreamId, id string) error
}

type Host struct {
	EtcdKey   string
	Name      string
	Locations []*Location
}

func (l *Host) String() string {
	return fmt.Sprintf("host(name=%s)", l.Name)
}

type Location struct {
	EtcdKey    string
	Path       string
	Name       string
	Upstream   *Upstream
	ConnLimits []*ConnLimit
	RateLimits []*RateLimit
}

func (l *Location) String() string {
	return fmt.Sprintf("location(id=%s, path=%s, ratelimits=%s, connlimits=%s)", l.Name, l.Path, l.RateLimits, l.ConnLimits)
}

type ConnLimit struct {
	EtcdKey     string
	Connections int
	Variable    string
}

type RateLimit struct {
	EtcdKey       string
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
	return fmt.Sprintf("ratelimit(var=%s, reqs/%s=%d, burst=%d)", rl.Variable, time.Duration(rl.PeriodSeconds)*time.Second, rl.Requests, rl.Burst)
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
	return fmt.Sprintf("connlimit(conn=%d, var=%s)", cl.Connections, cl.Variable)
}

type Upstream struct {
	EtcdKey   string
	Name      string
	Endpoints []*Endpoint
}

func (u *Upstream) String() string {
	return fmt.Sprintf("upstream(id=%s)", u.Name)
}

type Endpoint struct {
	EtcdKey string
	Name    string
	Path    string
	Url     string
	Stats   *EndpointStats
}

func (e *Endpoint) String() string {
	if e.Stats == nil {
		return fmt.Sprintf("endpoint(id=%s, url=%s)", e.Name, e.Url)
	} else {
		return fmt.Sprintf("endpoint(id=%s, url=%s, stats=%s)", e.Name, e.Url, e.Stats)
	}
}

type Change struct {
	Action string
	Parent interface{}
	Child  interface{}
	Keys   map[string]string
}

type EndpointStats struct {
	Successes     int64
	Failures      int64
	FailRate      float64
	PeriodSeconds int
}

func (e *EndpointStats) String() string {
	reqsSec := (e.Failures + e.Successes) / int64(e.PeriodSeconds)
	return fmt.Sprintf("(failRate=%.2f/%s, fail/success=%d/%d, freq=%d reqs/sec)", e.FailRate, time.Duration(e.PeriodSeconds)*time.Second, e.Failures, e.Successes, reqsSec)
}

type StatsGetter interface {
	GetStats(hostname string, locationId string, endpointId string) (*EndpointStats, error)
}
