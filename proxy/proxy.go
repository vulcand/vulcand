package proxy

type Proxy interface {
	AddServer(name string) error
	GetServers() ([]Server, error)
}

type Server struct {
	EtcdKey   string
	Name      string
	Locations []Location
}

type Location struct {
	EtcdKey  string
	Path     string
	Name     string
	Upstream Upstream
}

type Upstream struct {
	EtcdKey   string
	Name      string
	Endpoints []Endpoint
}

type Endpoint struct {
	EtcdKey string
	Path    string
	Url     string
}
