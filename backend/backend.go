package backend

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

type Location struct {
	EtcdKey  string
	Path     string
	Name     string
	Upstream *Upstream
}

type Upstream struct {
	EtcdKey   string
	Name      string
	Endpoints []*Endpoint
}

type Endpoint struct {
	EtcdKey string
	Name    string
	Path    string
	Url     string
}
