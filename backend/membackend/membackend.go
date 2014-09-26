// Provides in memory backend implementation, mostly used for test purposes
package membackend

import (
	"fmt"
	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/plugin"
	"sync/atomic"
)

type MemBackend struct {
	counter   int32
	Hosts     []*Host
	Upstreams []*Upstream
	Registry  *Registry
	ChangesC  chan interface{}
	ErrorsC   chan error
}

func NewMemBackend(r *Registry) *MemBackend {
	return &MemBackend{
		Hosts:     []*Host{},
		Upstreams: []*Upstream{},
		Registry:  r,
		ChangesC:  make(chan interface{}, 1000),
		ErrorsC:   make(chan error),
	}
}

func (m *MemBackend) Close() {
}

func (m *MemBackend) autoId() string {
	return fmt.Sprintf("%d", atomic.AddInt32(&m.counter, 1))
}

func (m *MemBackend) GetHosts() ([]*Host, error) {
	return m.Hosts, nil
}

func (m *MemBackend) GetRegistry() *Registry {
	return m.Registry
}

func (m *MemBackend) AddHost(h *Host) (*Host, error) {
	if h, _ := m.GetHost(h.Name); h != nil {
		return nil, &AlreadyExistsError{}
	}
	m.Hosts = append(m.Hosts, h)
	return h, nil
}

func (m *MemBackend) UpdateHostKeyPair(hostname string, keyPair *KeyPair) (*Host, error) {
	host, err := m.GetHost(hostname)
	if err != nil {
		return nil, err
	}
	host.KeyPair = keyPair
	return host, nil
}

func (m *MemBackend) GetHost(name string) (*Host, error) {
	for _, h := range m.Hosts {
		if h.Name == name {
			return h, nil
		}
	}
	return nil, &NotFoundError{}
}

func (m *MemBackend) DeleteHost(name string) error {
	for i, h := range m.Hosts {
		if h.Name == name {
			m.Hosts = append(m.Hosts[:i], m.Hosts[i+1:]...)
			return nil
		}
	}
	return &NotFoundError{}
}

func (m *MemBackend) AddHostListener(hostname string, listener *Listener) (*Listener, error) {
	host, err := m.GetHost(hostname)
	if err != nil {
		return nil, err
	}
	for _, l := range host.Listeners {
		if l.Address.Equals(listener.Address) {
			return nil, &AlreadyExistsError{
				Message: fmt.Sprintf("listener using the same address %s already exists: %s ", l.Address, l),
			}
		}
		if l.Id == listener.Id {
			return nil, &AlreadyExistsError{}
		}
	}

	if listener.Id == "" {
		listener.Id = m.autoId()
	}

	host.Listeners = append(host.Listeners, listener)
	return listener, nil
}

func (m *MemBackend) DeleteHostListener(hostname string, listenerId string) error {
	host, err := m.GetHost(hostname)
	if err != nil {
		return err
	}

	for i, l := range host.Listeners {
		if l.Id == listenerId {
			host.Listeners = append(host.Listeners[:i], host.Listeners[i+1:]...)
			return nil
		}
	}
	return &NotFoundError{}
}

func (m *MemBackend) GetLocation(hostname, id string) (*Location, error) {
	host, err := m.GetHost(hostname)
	if err != nil {
		return nil, &NotFoundError{}
	}
	for _, l := range host.Locations {
		if l.Id == id {
			return l, nil
		}
	}
	return nil, &NotFoundError{}
}

func (m *MemBackend) AddLocation(loc *Location) (*Location, error) {
	host, err := m.GetHost(loc.Hostname)
	if err != nil {
		return nil, &NotFoundError{}
	}
	if l, _ := m.GetLocation(loc.Hostname, loc.Id); l != nil {
		return nil, &AlreadyExistsError{}
	}
	up, _ := m.GetUpstream(loc.Upstream.Id)

	if up == nil {
		return nil, &NotFoundError{}
	}
	loc.Upstream = up

	if loc.Id == "" {
		loc.Id = m.autoId()
	}
	host.Locations = append(host.Locations, loc)

	return loc, nil
}

func (m *MemBackend) DeleteLocation(hostname, id string) error {
	host, err := m.GetHost(hostname)
	if err != nil {
		return &NotFoundError{}
	}
	for i, l := range host.Locations {
		if l.Id == id {
			host.Locations = append(host.Locations[:i], host.Locations[i+1:]...)
			return nil
		}
	}
	return &NotFoundError{}
}

func (m *MemBackend) UpdateLocationUpstream(hostname, id string, upstreamId string) (*Location, error) {
	loc, err := m.GetLocation(hostname, id)
	if err != nil {
		return nil, &NotFoundError{}
	}
	up, err := m.GetUpstream(upstreamId)
	if err != nil {
		return nil, &NotFoundError{}
	}
	loc.Upstream = up
	return loc, nil
}

func (m *MemBackend) UpdateLocationOptions(hostname, id string, o LocationOptions) (*Location, error) {
	loc, err := m.GetLocation(hostname, id)
	if err != nil {
		return nil, &NotFoundError{}
	}
	loc.Options = o
	return loc, nil
}

func (m *MemBackend) AddLocationMiddleware(hostname, locationId string, mi *MiddlewareInstance) (*MiddlewareInstance, error) {
	loc, err := m.GetLocation(hostname, locationId)
	if err != nil {
		return nil, &NotFoundError{}
	}
	if m, _ := m.GetLocationMiddleware(hostname, locationId, mi.Type, mi.Id); m != nil {
		return nil, &AlreadyExistsError{}
	}
	if mi.Id == "" {
		mi.Id = m.autoId()
	}
	loc.Middlewares = append(loc.Middlewares, mi)
	return mi, nil
}

func (m *MemBackend) UpdateLocationMiddleware(hostname, locationId string, mi *MiddlewareInstance) (*MiddlewareInstance, error) {
	loc, err := m.GetLocation(hostname, locationId)
	if err != nil {
		return nil, &NotFoundError{}
	}
	for i, mi := range loc.Middlewares {
		if mi.Id == mi.Id && mi.Type == mi.Type {
			loc.Middlewares[i] = mi
			return mi, nil
		}
	}
	return nil, &NotFoundError{}
}

func (m *MemBackend) GetLocationMiddleware(hostname, locationId string, mType, id string) (*MiddlewareInstance, error) {
	loc, err := m.GetLocation(hostname, locationId)
	if err != nil {
		return nil, &NotFoundError{}
	}
	for _, mi := range loc.Middlewares {
		if mi.Id == id && mi.Type == mType {
			return mi, nil
		}
	}
	return nil, &NotFoundError{}
}

func (m *MemBackend) DeleteLocationMiddleware(hostname, locationId, mType, id string) error {
	loc, err := m.GetLocation(hostname, locationId)
	if err != nil {
		return &NotFoundError{}
	}
	for i, mi := range loc.Middlewares {
		if mi.Id == id && mi.Type == mType {
			loc.Middlewares = append(loc.Middlewares[:i], loc.Middlewares[i+1:]...)
			return nil
		}
	}
	return &NotFoundError{}
}

func (m *MemBackend) GetUpstreams() ([]*Upstream, error) {
	return m.Upstreams, nil
}

func (m *MemBackend) GetUpstream(id string) (*Upstream, error) {
	for _, up := range m.Upstreams {
		if up.Id == id {
			return up, nil
		}
	}
	return nil, &NotFoundError{}
}

func (m *MemBackend) AddUpstream(up *Upstream) (*Upstream, error) {
	if u, _ := m.GetUpstream(up.Id); u != nil {
		return nil, &AlreadyExistsError{}
	}
	if up.Id == "" {
		up.Id = m.autoId()
	}
	m.Upstreams = append(m.Upstreams, up)
	return up, nil
}

func (m *MemBackend) DeleteUpstream(id string) error {
	for i, u := range m.Upstreams {
		if u.Id == id {
			m.Upstreams = append(m.Upstreams[:i], m.Upstreams[i+1:]...)
			return nil
		}
	}
	return &NotFoundError{}
}

func (m *MemBackend) AddEndpoint(e *Endpoint) (*Endpoint, error) {
	if e, _ := m.GetEndpoint(e.UpstreamId, e.Id); e != nil {
		return nil, &AlreadyExistsError{}
	}
	u, err := m.GetUpstream(e.UpstreamId)
	if err != nil {
		return nil, &NotFoundError{}
	}
	if e.Id == "" {
		e.Id = m.autoId()
	}
	u.Endpoints = append(u.Endpoints, e)
	return e, nil
}

func (m *MemBackend) GetEndpoint(upstreamId, id string) (*Endpoint, error) {
	u, err := m.GetUpstream(upstreamId)
	if err != nil {
		return nil, &NotFoundError{}
	}
	for _, e := range u.Endpoints {
		if e.Id == id {
			return e, nil
		}
	}
	return nil, &NotFoundError{}
}

func (m *MemBackend) DeleteEndpoint(upstreamId, id string) error {
	u, err := m.GetUpstream(upstreamId)
	if err != nil {
		return &NotFoundError{}
	}
	for i, e := range u.Endpoints {
		if e.Id == id {
			u.Endpoints = append(u.Endpoints[:i], u.Endpoints[i+1:]...)
			return nil
		}
	}
	return &NotFoundError{}
}

func (m *MemBackend) WatchChanges(changes chan interface{}, cancel chan bool) error {
	for {
		select {
		case <-cancel:
			return nil
		case change := <-m.ChangesC:
			select {
			case changes <- change:
			case err := <-m.ErrorsC:
				return err
			}
		case err := <-m.ErrorsC:
			return err
		}
	}
}
