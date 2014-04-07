package backend

import (
	"fmt"
)

type Backend interface {
	GetHosts() ([]*Host, error)

	AddHost(name string) error
	DeleteHost(name string) error

	AddLocation(id, hostname, path, upstream string) error
	DeleteLocation(hostname, id string) error

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
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("endpoint(id=%s, url=%s)", e.Name, e.Url)
}

type Change struct {
	Action string
	Parent interface{}
	Child  interface{}
}
