package backend

import (
	"fmt"
)

type Backend interface {
	GetHosts() ([]*Host, error)

	AddHost(name string) error
	DeleteHost(name string) error

	AddLocation(hostname, id, path, upstream string) error
	DeleteLocation(hostname, id string) error
	UpdateLocationUpstream(hostname, id string, upstream string) error

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
	EtcdKey  string
	Path     string
	Name     string
	Upstream *Upstream
}

func (l *Location) String() string {
	return fmt.Sprintf("location(id=%s, path=%s)", l.Name, l.Path)
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
	return fmt.Sprintf("(window=%dsec, failRate=%.2f, failures=%d, successes=%d, freq=%d reqs/sec)", e.PeriodSeconds, e.FailRate, e.Failures, e.Successes, reqsSec)
}

type StatsGetter interface {
	GetStats(hostname string, locationId string, endpointId string) (*EndpointStats, error)
}
