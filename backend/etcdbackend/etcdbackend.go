// Etcd implementation of the backend, where all vulcand properties are implemented as directories or keys.
// This backend watches the changes and generates events with sequence of changes.
package etcdbackend

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/secret"
)

const reconnectTimeout = 3 * time.Second

type EtcdBackend struct {
	nodes    []string
	registry *plugin.Registry
	etcdKey  string
	client   *etcd.Client
	cancelC  chan bool
	stopC    chan bool

	options Options
}

type Options struct {
	EtcdConsistency string
	Box             *secret.Box
	TimeProvider    timetools.TimeProvider
}

func NewEtcdBackend(registry *plugin.Registry, nodes []string, etcdKey string) (*EtcdBackend, error) {
	return NewEtcdBackendWithOptions(registry, nodes, etcdKey, Options{})
}

func NewEtcdBackendWithOptions(registry *plugin.Registry, nodes []string, etcdKey string, options Options) (*EtcdBackend, error) {
	o, err := parseOptions(options)
	if err != nil {
		return nil, err
	}
	b := &EtcdBackend{
		nodes:    nodes,
		registry: registry,
		etcdKey:  etcdKey,
		cancelC:  make(chan bool, 1),
		stopC:    make(chan bool, 1),
		options:  o,
	}
	if err := b.reconnect(); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *EtcdBackend) reconnect() error {
	if s.client != nil {
		s.client.Close()
	}
	client := etcd.NewClient(s.nodes)
	if err := client.SetConsistency(s.options.EtcdConsistency); err != nil {
		return err
	}
	s.client = client
	return nil
}

func (s *EtcdBackend) GetRegistry() *plugin.Registry {
	return s.registry
}

func (s *EtcdBackend) GetHosts() ([]*backend.Host, error) {
	return s.readHosts(true)
}

func (s *EtcdBackend) UpdateHostCertificate(hostname string, cert *backend.Certificate) (*backend.Host, error) {
	host, err := s.GetHost(hostname)
	if err != nil {
		return nil, err
	}
	if err := s.setHostCert(host.Name, cert); err != nil {
		return nil, err
	}
	host.Cert = cert
	return host, nil
}

func (s *EtcdBackend) AddHost(h *backend.Host) (*backend.Host, error) {
	if err := s.createDir(s.path("hosts", h.Name)); err != nil {
		return nil, err
	}
	if h.Cert != nil {
		if err := s.setHostCert(h.Name, h.Cert); err != nil {
			return nil, err
		}
	}
	if len(h.Listeners) != 0 {
		for _, l := range h.Listeners {
			if _, err := s.AddHostListener(h.Name, l); err != nil {
				return nil, err
			}
		}
	}
	return h, nil
}

func (s *EtcdBackend) AddHostListener(hostname string, listener *backend.Listener) (*backend.Listener, error) {
	host, err := s.GetHost(hostname)
	if err != nil {
		return nil, err
	}
	for _, l := range host.Listeners {
		if l.Address.Equals(listener.Address) {
			return nil, &backend.AlreadyExistsError{
				Message: fmt.Sprintf("listener using the same address %s already exists: %s ", l.Address, l),
			}
		}
	}

	if listener.Id == "" {
		id, err := s.addChildJSONVal(s.path("hosts", hostname, "listeners"), listener)
		if err != nil {
			return nil, err
		}
		listener.Id = id
	} else {
		if err := s.setJSONVal(s.path("hosts", hostname, "listeners", listener.Id), listener); err != nil {
			return nil, err
		}
	}
	return listener, nil
}

func (s *EtcdBackend) DeleteHostListener(hostname string, listenerId string) error {
	return s.deleteKey(s.path("hosts", hostname, "listeners", listenerId))
}

func (s *EtcdBackend) GetHost(hostname string) (*backend.Host, error) {
	return s.readHost(hostname, true)
}

func (s *EtcdBackend) setHostCert(hostname string, cert *backend.Certificate) error {
	bytes, err := json.Marshal(cert)
	if err != nil {
		return err
	}
	return s.setSealedVal(s.path("hosts", hostname, "cert"), bytes)
}

func (s *EtcdBackend) readHostCert(hostname string) (*backend.Certificate, error) {
	if s.options.Box == nil {
		return nil, nil
	}
	bytes, err := s.getSealedVal(s.path("hosts", hostname, "cert"))
	if err != nil {
		return nil, err
	}
	var cert *backend.Certificate
	if err := json.Unmarshal(bytes, &cert); err != nil {
		return nil, err
	}
	return cert, nil
}

func (s *EtcdBackend) readHost(hostname string, deep bool) (*backend.Host, error) {
	hostKey := s.path("hosts", hostname)
	if err := s.checkKeyExists(hostKey); err != nil {
		return nil, err
	}

	cert, err := s.readHostCert(hostname)
	if err != nil && !isNotFoundError(err) {
		return nil, err
	}

	host := &backend.Host{
		Name:      hostname,
		Locations: []*backend.Location{},
		Cert:      cert,
		Listeners: []*backend.Listener{},
	}

	listeners, err := s.getVals(hostKey, "listeners")
	if err != nil {
		return nil, err
	}
	for _, p := range listeners {
		l, err := backend.ListenerFromJson([]byte(p.Val))
		if err != nil {
			return nil, err
		}
		l.Id = suffix(p.Key)
		host.Listeners = append(host.Listeners, l)
	}

	if !deep {
		return host, nil
	}

	for _, key := range s.getDirs(hostKey, "locations") {
		location, err := s.GetLocation(hostname, suffix(key))
		if err != nil {
			return nil, err
		}
		host.Locations = append(host.Locations, location)
	}
	return host, nil
}

func (s *EtcdBackend) DeleteHost(name string) error {
	return s.deleteKey(s.path("hosts", name))
}

func (s *EtcdBackend) AddLocation(l *backend.Location) (*backend.Location, error) {
	if _, err := s.GetUpstream(l.Upstream.Id); err != nil {
		return nil, err
	}

	// Check if the host of the location exists
	if _, err := s.readHost(l.Hostname, false); err != nil {
		return nil, err
	}

	// Auto generate id if not set by user, very handy feature
	if l.Id == "" {
		id, err := s.addChildDir(s.path("hosts", l.Hostname, "locations"))
		if err != nil {
			return nil, err
		}
		l.Id = id
	} else {
		if err := s.createDir(s.path("hosts", l.Hostname, "locations", l.Id)); err != nil {
			return nil, err
		}
	}
	locationKey := s.path("hosts", l.Hostname, "locations", l.Id)
	if err := s.setJSONVal(join(locationKey, "options"), l.Options); err != nil {
		return nil, err
	}
	if err := s.setStringVal(join(locationKey, "path"), l.Path); err != nil {
		return nil, err
	}
	if err := s.setStringVal(join(locationKey, "upstream"), l.Upstream.Id); err != nil {
		return nil, err
	}
	return l, nil
}

func (s *EtcdBackend) ExpectLocation(hostname, locationId string) error {
	return s.checkKeyExists(s.path("hosts", hostname, "locations", locationId))
}

func (s *EtcdBackend) GetLocation(hostname, locationId string) (*backend.Location, error) {
	locationKey := s.path("hosts", hostname, "locations", locationId)

	if err := s.checkKeyExists(locationKey); err != nil {
		return nil, err
	}

	path, err := s.getVal(join(locationKey, "path"))
	if err != nil {
		return nil, err
	}

	upstreamKey, err := s.getVal(join(locationKey, "upstream"))
	if err != nil {
		return nil, err
	}

	var options *backend.LocationOptions
	err = s.getJSONVal(join(locationKey, "options"), &options)
	if err != nil {
		if isNotFoundError(err) {
			options = &backend.LocationOptions{}
		} else {
			return nil, err
		}
	}

	location := &backend.Location{
		Hostname:    hostname,
		Id:          locationId,
		Path:        path,
		Middlewares: []*backend.MiddlewareInstance{},
		Options:     *options,
	}
	upstream, err := s.GetUpstream(upstreamKey)
	if err != nil {
		return nil, err
	}
	for _, spec := range s.registry.GetSpecs() {
		values, err := s.getVals(locationKey, "middlewares", spec.Type)
		if err != nil {
			return nil, err
		}
		for _, cl := range values {
			m, err := s.GetLocationMiddleware(hostname, locationId, spec.Type, suffix(cl.Key))
			if err != nil {
				log.Errorf("failed to read middleware %s(%s), error: %s", spec.Type, cl.Key, err)
			} else {
				location.Middlewares = append(location.Middlewares, m)
			}
		}
	}

	location.Upstream = upstream
	return location, nil
}

func (s *EtcdBackend) UpdateLocationUpstream(hostname, id, upstreamId string) (*backend.Location, error) {
	// Make sure upstream exists
	if _, err := s.GetUpstream(upstreamId); err != nil {
		return nil, err
	}

	if err := s.setStringVal(s.path("hosts", hostname, "locations", id, "upstream"), upstreamId); err != nil {
		return nil, err
	}

	return s.GetLocation(hostname, id)
}

func (s *EtcdBackend) UpdateLocationOptions(hostname, id string, o backend.LocationOptions) (*backend.Location, error) {
	if err := s.setJSONVal(s.path("hosts", hostname, "locations", id, "options"), o); err != nil {
		return nil, err
	}
	return s.GetLocation(hostname, id)
}

func (s *EtcdBackend) DeleteLocation(hostname, id string) error {
	return s.deleteKey(s.path("hosts", hostname, "locations", id))
}

func (s *EtcdBackend) AddUpstream(u *backend.Upstream) (*backend.Upstream, error) {
	if u.Id == "" {
		id, err := s.addChildDir(s.path("upstreams"))
		if err != nil {
			return nil, err
		}
		u.Id = id
	} else {
		if err := s.createDir(s.path("upstreams", u.Id)); err != nil {
			return nil, err
		}
	}
	return u, nil
}

func (s *EtcdBackend) GetUpstream(upstreamId string) (*backend.Upstream, error) {
	upstreamKey := s.path("upstreams", upstreamId)

	if err := s.checkKeyExists(upstreamKey); err != nil {
		return nil, err
	}

	upstream := &backend.Upstream{
		Id:        suffix(upstreamKey),
		Endpoints: []*backend.Endpoint{},
	}

	endpointPairs, err := s.getVals(join(upstreamKey, "endpoints"))
	if err != nil {
		return nil, err
	}
	for _, e := range endpointPairs {
		_, err := endpoint.ParseUrl(e.Val)
		if err != nil {
			continue
		}
		e := &backend.Endpoint{
			Url:        e.Val,
			Id:         suffix(e.Key),
			UpstreamId: upstreamId,
		}
		upstream.Endpoints = append(upstream.Endpoints, e)
	}
	return upstream, nil
}

func (s *EtcdBackend) GetUpstreams() ([]*backend.Upstream, error) {
	upstreams := []*backend.Upstream{}
	for _, upstreamKey := range s.getDirs(s.etcdKey, "upstreams") {
		upstream, err := s.GetUpstream(suffix(upstreamKey))
		if err != nil {
			return nil, err
		}
		upstreams = append(upstreams, upstream)
	}
	return upstreams, nil
}

func (s *EtcdBackend) DeleteUpstream(upstreamId string) error {
	locations, err := s.upstreamUsedBy(upstreamId)
	if err != nil {
		return err
	}
	if len(locations) != 0 {
		return fmt.Errorf("can not delete upstream '%s', it is in use by %s", locations)
	}
	_, err = s.client.Delete(s.path("upstreams", upstreamId), true)
	return convertErr(err)
}

func (s *EtcdBackend) AddEndpoint(e *backend.Endpoint) (*backend.Endpoint, error) {
	if e.Id == "" {
		id, err := s.addChildStringVal(s.path("upstreams", e.UpstreamId, "endpoints"), e.Url)
		if err != nil {
			return nil, err
		}
		e.Id = id
	} else {
		if err := s.setStringVal(s.path("upstreams", e.UpstreamId, "endpoints", e.Id), e.Url); err != nil {
			return nil, err
		}
	}
	return e, nil
}

func (s *EtcdBackend) GetEndpoint(upstreamId, id string) (*backend.Endpoint, error) {
	if _, err := s.GetUpstream(upstreamId); err != nil {
		return nil, err
	}

	url, err := s.getVal(s.path("upstreams", upstreamId, "endpoints", id))
	if err != nil {
		return nil, err
	}

	return &backend.Endpoint{
		Url:        url,
		Id:         id,
		UpstreamId: upstreamId,
	}, nil
}

func (s *EtcdBackend) DeleteEndpoint(upstreamId, id string) error {
	if _, err := s.GetUpstream(upstreamId); err != nil {
		return err
	}
	return s.deleteKey(s.path("upstreams", upstreamId, "endpoints", id))
}

func (s *EtcdBackend) AddLocationMiddleware(hostname, locationId string, m *backend.MiddlewareInstance) (*backend.MiddlewareInstance, error) {
	if err := s.ExpectLocation(hostname, locationId); err != nil {
		return nil, err
	}
	if m.Id == "" {
		id, err := s.addChildJSONVal(s.path("hosts", hostname, "locations", locationId, "middlewares", m.Type), m)
		if err != nil {
			return nil, err
		}
		m.Id = id
	} else {
		if err := s.setJSONVal(s.path("hosts", hostname, "locations", locationId, "middlewares", m.Type, m.Id), m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (s *EtcdBackend) GetLocationMiddleware(hostname, locationId, mType, id string) (*backend.MiddlewareInstance, error) {
	if err := s.ExpectLocation(hostname, locationId); err != nil {
		return nil, err
	}
	backendKey := s.path("hosts", hostname, "locations", locationId, "middlewares", mType, id)
	bytes, err := s.getVal(backendKey)
	if err != nil {
		return nil, err
	}
	out, err := backend.MiddlewareFromJson([]byte(bytes), s.registry.GetSpec)
	if err != nil {
		return nil, err
	}
	out.Id = id
	return out, nil
}

func (s *EtcdBackend) UpdateLocationMiddleware(hostname, locationId string, m *backend.MiddlewareInstance) (*backend.MiddlewareInstance, error) {
	if len(m.Id) == 0 || len(hostname) == 0 || len(locationId) == 0 {
		return nil, fmt.Errorf("provide hostname, location and middleware id to update")
	}
	spec := s.registry.GetSpec(m.Type)
	if spec == nil {
		return nil, fmt.Errorf("middleware type %s is not registered", m.Type)
	}
	if err := s.ExpectLocation(hostname, locationId); err != nil {
		return nil, err
	}
	if err := s.setJSONVal(s.path("hosts", hostname, "locations", locationId, "middlewares", m.Type, m.Id), m); err != nil {
		return m, err
	}
	return m, nil
}

func (s *EtcdBackend) DeleteLocationMiddleware(hostname, locationId, mType, id string) error {
	if err := s.ExpectLocation(hostname, locationId); err != nil {
		return err
	}
	return s.deleteKey(s.path("hosts", hostname, "locations", locationId, "middlewares", mType, id))
}

// Watches etcd changes and generates structured events telling vulcand to add or delete locations, hosts etc.
// if initialSetup is true, reads the existing configuration and generates events for inital configuration of the proxy.
func (s *EtcdBackend) WatchChanges(changes chan interface{}, initialSetup bool) error {
	if initialSetup == true {
		log.Infof("Etcd backend reading initial configuration, etcd nodes: %s", s.nodes)
		if err := s.generateChanges(changes); err != nil {
			log.Errorf("Failed to generate changes: %s, stopping watch.", err)
			return err
		}
	}
	// This index helps us to get changes in sequence, as they were performed by clients.
	waitIndex := uint64(0)
	for {
		response, err := s.client.Watch(s.etcdKey, waitIndex, true, nil, s.cancelC)
		if err != nil {
			switch err {
			case etcd.ErrWatchStoppedByUser:
				log.Infof("Stop watching: graceful shutdown")
				return nil
			default:
				log.Errorf("Unexpected error: %s, reconnecting", err)
				s.options.TimeProvider.Sleep(reconnectTimeout)
				s.reconnect()
				continue
			}
		}
		waitIndex = response.Node.ModifiedIndex + 1
		log.Infof("%s", responseToString(response))
		change, err := s.parseChange(response)
		if err != nil {
			log.Warningf("Ignore '%s', error: %s", responseToString(response), err)
			continue
		}
		if change != nil {
			log.Infof("%s", change)
			select {
			case changes <- change:
			case <-s.stopC:
				return nil
			}
		}
	}
	return nil
}

func (s *EtcdBackend) StopWatching() {
	s.cancelC <- true
	s.stopC <- true
}

// Reads the configuration of the vulcand and generates a sequence of events
// just like as someone was creating locations and hosts in sequence.
func (s *EtcdBackend) generateChanges(changes chan interface{}) error {
	upstreams, err := s.GetUpstreams()
	if err != nil {
		return err
	}

	if len(upstreams) == 0 {
		log.Warningf("No upstreams found")
	}

	for _, u := range upstreams {
		changes <- &backend.UpstreamAdded{
			Upstream: u,
		}
		for _, e := range u.Endpoints {
			changes <- &backend.EndpointAdded{
				Upstream: u,
				Endpoint: e,
			}
		}
	}

	hosts, err := s.readHosts(true)
	if err != nil {
		return err
	}

	if len(hosts) == 0 {
		log.Warningf("No hosts found")
	}

	for _, h := range hosts {
		changes <- &backend.HostAdded{
			Host: h,
		}
		for _, l := range h.Locations {
			changes <- &backend.LocationAdded{
				Host:     h,
				Location: l,
			}
		}
	}
	return nil
}

type MatcherFn func(*etcd.Response) (interface{}, error)

// Dispatches etcd key changes changes to the etcd to the matching functions
func (s *EtcdBackend) parseChange(response *etcd.Response) (interface{}, error) {
	matchers := []MatcherFn{
		s.parseHostChange,
		s.parseHostCertChange,
		s.parseHostListenerChange,
		s.parseLocationChange,
		s.parseLocationUpstreamChange,
		s.parseLocationOptionsChange,
		s.parseLocationPathChange,
		s.parseUpstreamChange,
		s.parseUpstreamEndpointChange,
		s.parseUpstreamChange,
		s.parseMiddlewareChange,
	}
	for _, matcher := range matchers {
		a, err := matcher(response)
		if a != nil || err != nil {
			return a, err
		}
	}
	return nil, nil
}

func (s *EtcdBackend) parseHostChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/hosts/([^/]+)$").FindStringSubmatch(response.Node.Key)
	if len(out) != 2 {
		return nil, nil
	}

	hostname := out[1]

	if response.Action == "create" {
		host, err := s.readHost(hostname, false)
		if err != nil {
			return nil, err
		}
		return &backend.HostAdded{
			Host: host,
		}, nil
	} else if response.Action == "delete" {
		return &backend.HostDeleted{
			Name: hostname,
		}, nil
	}
	return nil, fmt.Errorf("unsupported action on the location: %s", response.Action)
}

func (s *EtcdBackend) parseHostCertChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/hosts/([^/]+)/cert").FindStringSubmatch(response.Node.Key)
	if len(out) != 2 {
		return nil, nil
	}

	if response.Action != "create" && response.Action != "set" && response.Action != "delete" {
		return nil, fmt.Errorf("unsupported action on the certificate: %s", response.Action)
	}
	hostname := out[1]
	host, err := s.readHost(hostname, false)
	if err != nil {
		return nil, err
	}
	return &backend.HostCertUpdated{
		Host: host,
	}, nil
}

func (s *EtcdBackend) parseLocationChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/hosts/([^/]+)/locations/([^/]+)$").FindStringSubmatch(response.Node.Key)
	if len(out) != 3 {
		return nil, nil
	}
	hostname, locationId := out[1], out[2]
	host, err := s.readHost(hostname, false)
	if err != nil {
		return nil, err
	}
	if response.Action == "create" {
		location, err := s.GetLocation(hostname, locationId)
		if err != nil {
			return nil, err
		}
		return &backend.LocationAdded{
			Host:     host,
			Location: location,
		}, nil
	} else if response.Action == "delete" {
		return &backend.LocationDeleted{
			Host:       host,
			LocationId: locationId,
		}, nil
	}
	return nil, fmt.Errorf("unsupported action on the location: %s", response.Action)
}

func (s *EtcdBackend) parseLocationUpstreamChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/hosts/([^/]+)/locations/([^/]+)/upstream").FindStringSubmatch(response.Node.Key)
	if len(out) != 3 {
		return nil, nil
	}

	if response.Action != "create" && response.Action != "set" {
		return nil, fmt.Errorf("unsupported action on the location upstream: %s", response.Action)
	}

	hostname, locationId := out[1], out[2]
	host, err := s.readHost(hostname, false)
	if err != nil {
		return nil, err
	}
	location, err := s.GetLocation(hostname, locationId)
	if err != nil {
		return nil, err
	}
	return &backend.LocationUpstreamUpdated{
		Host:     host,
		Location: location,
	}, nil
}

func (s *EtcdBackend) parseLocationOptionsChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/hosts/([^/]+)/locations/([^/]+)/options").FindStringSubmatch(response.Node.Key)
	if len(out) != 3 {
		return nil, nil
	}

	if response.Action != "create" && response.Action != "set" {
		return nil, fmt.Errorf("unsupported action on the location options: %s", response.Action)
	}

	hostname, locationId := out[1], out[2]
	host, err := s.readHost(hostname, false)
	if err != nil {
		return nil, err
	}
	location, err := s.GetLocation(hostname, locationId)
	if err != nil {
		return nil, err
	}
	return &backend.LocationOptionsUpdated{
		Host:     host,
		Location: location,
	}, nil
}

func (s *EtcdBackend) parseLocationPathChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/hosts/([^/]+)/locations/([^/]+)/path").FindStringSubmatch(response.Node.Key)
	if len(out) != 3 {
		return nil, nil
	}

	if response.Action != "create" && response.Action != "set" {
		return nil, fmt.Errorf("unsupported action on the location path: %s", response.Action)
	}

	hostname, locationId := out[1], out[2]
	host, err := s.readHost(hostname, false)
	if err != nil {
		return nil, err
	}
	location, err := s.GetLocation(hostname, locationId)
	if err != nil {
		return nil, err
	}

	return &backend.LocationPathUpdated{
		Host:     host,
		Location: location,
		Path:     response.Node.Value,
	}, nil
}

func (s *EtcdBackend) parseHostListenerChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/hosts/([^/]+)/listeners/([^/]+)").FindStringSubmatch(response.Node.Key)
	if len(out) != 3 {
		return nil, nil
	}
	hostname, listenerId := out[1], out[2]

	host, err := s.readHost(hostname, false)
	if err != nil {
		return nil, err
	}
	if response.Action == "create" || response.Action == "set" {
		for _, l := range host.Listeners {
			if l.Id == listenerId {
				return &backend.HostListenerAdded{
					Host:     host,
					Listener: l,
				}, nil
			}
		}
		return nil, fmt.Errorf("listener %s not found", listenerId)
	} else if response.Action == "delete" {
		return &backend.HostListenerDeleted{
			Host:       host,
			ListenerId: listenerId,
		}, nil
	}
	return nil, fmt.Errorf("unsupported action on the listener: %s", response.Action)
}

func (s *EtcdBackend) parseUpstreamChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/upstreams/([^/]+)$").FindStringSubmatch(response.Node.Key)
	if len(out) != 2 {
		return nil, nil
	}
	upstreamId := out[1]
	if response.Action == "create" {
		upstream, err := s.GetUpstream(upstreamId)
		if err != nil {
			return nil, err
		}
		return &backend.UpstreamAdded{
			Upstream: upstream,
		}, nil
	} else if response.Action == "delete" {
		return &backend.UpstreamDeleted{
			UpstreamId: upstreamId,
		}, nil
	}
	return nil, fmt.Errorf("unsupported node action: %s", response)
}

func (s *EtcdBackend) parseUpstreamEndpointChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/upstreams/([^/]+)/endpoints/([^/]+)").FindStringSubmatch(response.Node.Key)
	if len(out) != 3 {
		return nil, nil
	}
	upstreamId, endpointId := out[1], out[2]
	upstream, err := s.GetUpstream(upstreamId)
	if err != nil {
		return nil, err
	}

	affectedLocations, err := s.upstreamUsedBy(upstreamId)
	if err != nil {
		return nil, err
	}

	if response.Action == "create" {
		for _, e := range upstream.Endpoints {
			if e.Id == endpointId {
				return &backend.EndpointAdded{
					Upstream:          upstream,
					Endpoint:          e,
					AffectedLocations: affectedLocations,
				}, nil
			}
		}
		return nil, fmt.Errorf("endpoint %s not found", endpointId)
	} else if response.Action == "set" {
		for _, e := range upstream.Endpoints {
			if e.Id == endpointId {
				return &backend.EndpointUpdated{
					Upstream:          upstream,
					Endpoint:          e,
					AffectedLocations: affectedLocations,
				}, nil
			}
		}
		return nil, fmt.Errorf("endpoint %s not found", endpointId)
	} else if response.Action == "delete" {
		return &backend.EndpointDeleted{
			Upstream:          upstream,
			EndpointId:        endpointId,
			AffectedLocations: affectedLocations,
		}, nil
	}
	return nil, fmt.Errorf("unsupported action on the endpoint: %s", response.Action)
}

func (s *EtcdBackend) parseMiddlewareChange(response *etcd.Response) (interface{}, error) {
	out := regexp.MustCompile("/hosts/([^/]+)/locations/([^/]+)/middlewares/([^/]+)").FindStringSubmatch(response.Node.Key)
	if len(out) != 4 {
		return nil, nil
	}
	hostname, locationId, mType := out[1], out[2], out[3]
	mId := suffix(response.Node.Key)

	spec := s.registry.GetSpec(mType)
	if spec == nil {
		return nil, fmt.Errorf("unregistered middleware type %s", mType)
	}
	host, err := s.readHost(hostname, false)
	if err != nil {
		return nil, err
	}
	location, err := s.GetLocation(hostname, locationId)
	if err != nil {
		return nil, err
	}
	if response.Action == "create" {
		m, err := s.GetLocationMiddleware(hostname, locationId, mType, mId)
		if err != nil {
			return nil, err
		}
		return &backend.LocationMiddlewareAdded{
			Host:       host,
			Location:   location,
			Middleware: m,
		}, nil
	} else if response.Action == "set" {
		m, err := s.GetLocationMiddleware(hostname, locationId, mType, mId)
		if err != nil {
			return nil, err
		}
		return &backend.LocationMiddlewareUpdated{
			Host:       host,
			Location:   location,
			Middleware: m,
		}, nil
	} else if response.Action == "delete" {
		return &backend.LocationMiddlewareDeleted{
			Host:           host,
			Location:       location,
			MiddlewareId:   mId,
			MiddlewareType: mType,
		}, nil
	}
	return nil, fmt.Errorf("unsupported action on the rate: %s", response.Action)
}

func (s *EtcdBackend) readHosts(deep bool) ([]*backend.Host, error) {
	hosts := []*backend.Host{}
	for _, hostKey := range s.getDirs(s.etcdKey, "hosts") {
		host, err := s.readHost(suffix(hostKey), deep)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func (s *EtcdBackend) upstreamUsedBy(upstreamId string) ([]*backend.Location, error) {
	locations := []*backend.Location{}
	hosts, err := s.readHosts(true)
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		for _, l := range h.Locations {
			if l.Upstream.Id == upstreamId {
				locations = append(locations, l)
			}
		}
	}
	return locations, nil
}

func (s EtcdBackend) path(keys ...string) string {
	return strings.Join(append([]string{s.etcdKey}, keys...), "/")
}

func (s *EtcdBackend) setSealedVal(key string, val []byte) error {
	if s.options.Box == nil {
		return fmt.Errorf("this backend does not support encryption")
	}
	v, err := s.options.Box.Seal([]byte(val))
	if err != nil {
		return err
	}
	bytes, err := sealedValueToJSON(v)
	if err != nil {
		return err
	}
	return s.setVal(key, bytes)
}

func (s *EtcdBackend) setJSONVal(key string, v interface{}) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.setVal(key, bytes)
}

func (s *EtcdBackend) setVal(key string, val []byte) error {
	_, err := s.client.Set(key, string(val), 0)
	return convertErr(err)
}

func (s *EtcdBackend) setStringVal(key string, val string) error {
	return s.setVal(key, []byte(val))
}

func (s *EtcdBackend) getSealedVal(key string) ([]byte, error) {
	if s.options.Box == nil {
		return nil, fmt.Errorf("this backend does not support encryption")
	}
	bytes, err := s.getVal(key)
	if err != nil {
		return nil, err
	}
	sv, err := sealedValueFromJSON([]byte(bytes))
	if err != nil {
		return nil, err
	}
	return s.options.Box.Open(sv)
}

func (s *EtcdBackend) getJSONVal(key string, in interface{}) error {
	val, err := s.getVal(key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), in)
}

func (s *EtcdBackend) getVal(key string) (string, error) {
	response, err := s.client.Get(key, false, false)
	if err != nil {
		return "", convertErr(err)
	}

	if isDir(response.Node) {
		return "", &backend.NotFoundError{Message: fmt.Sprintf("missing key: %s", key)}
	}
	return response.Node.Value, nil
}

func (s *EtcdBackend) getDirs(keys ...string) []string {
	var out []string
	response, err := s.client.Get(strings.Join(keys, "/"), true, true)
	if notFound(err) {
		return out
	}

	if response == nil || !isDir(response.Node) {
		return out
	}

	for _, srvNode := range response.Node.Nodes {
		if isDir(&srvNode) {
			out = append(out, srvNode.Key)
		}
	}
	return out
}

func (s *EtcdBackend) getVals(keys ...string) ([]Pair, error) {
	var out []Pair
	response, err := s.client.Get(strings.Join(keys, "/"), true, true)
	if err != nil {
		if notFound(err) {
			return out, nil
		}
		return nil, err
	}

	if !isDir(response.Node) {
		return out, nil
	}

	for _, srvNode := range response.Node.Nodes {
		if !isDir(&srvNode) {
			out = append(out, Pair{srvNode.Key, srvNode.Value})
		}
	}
	return out, nil
}

func (s *EtcdBackend) createDir(key string) error {
	_, err := s.client.CreateDir(key, 0)
	return convertErr(err)
}

func (s *EtcdBackend) addChildDir(key string) (string, error) {
	response, err := s.client.AddChildDir(key, 0)
	if err != nil {
		return "", convertErr(err)
	}
	return suffix(response.Node.Key), nil
}

func (s *EtcdBackend) addChildVal(key string, val []byte) (string, error) {
	response, err := s.client.AddChild(key, string(val), 0)
	if err != nil {
		return "", convertErr(err)
	}
	return suffix(response.Node.Key), nil
}

func (s *EtcdBackend) addChildJSONVal(key string, v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return s.addChildVal(key, bytes)
}

func (s *EtcdBackend) addChildStringVal(key string, val string) (string, error) {
	return s.addChildVal(key, []byte(val))
}

func (s *EtcdBackend) checkKeyExists(key string) error {
	_, err := s.client.Get(key, false, false)
	return convertErr(err)
}

func (s *EtcdBackend) deleteKey(key string) error {
	_, err := s.client.Delete(key, true)
	return convertErr(err)
}

type Pair struct {
	Key string
	Val string
}

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[len(vals)-1]
}

func join(keys ...string) string {
	return strings.Join(keys, "/")
}

func notFound(e error) bool {
	err, ok := e.(*etcd.EtcdError)
	return ok && err.ErrorCode == 100
}

func convertErr(e error) error {
	if e == nil {
		return nil
	}
	switch err := e.(type) {
	case *etcd.EtcdError:
		if err.ErrorCode == 100 {
			return &backend.NotFoundError{Message: err.Message}
		}
		if err.ErrorCode == 105 {
			return &backend.AlreadyExistsError{Message: err.Message}
		}
	}
	return e
}

func isDir(n *etcd.Node) bool {
	return n != nil && n.Dir == true
}

func parseOptions(o Options) (Options, error) {
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	if o.EtcdConsistency == "" {
		o.EtcdConsistency = etcd.STRONG_CONSISTENCY
	}
	return o, nil
}

type sealedValue struct {
	Encryption string
	Value      secret.SealedBytes
}

func sealedValueToJSON(b *secret.SealedBytes) ([]byte, error) {
	data := &sealedValue{
		Encryption: encryptionSecretBox,
		Value:      *b,
	}
	return json.Marshal(&data)
}

func sealedValueFromJSON(bytes []byte) (*secret.SealedBytes, error) {
	var v *sealedValue
	if err := json.Unmarshal(bytes, &v); err != nil {
		return nil, err
	}
	if v.Encryption != encryptionSecretBox {
		return nil, fmt.Errorf("unsupported encryption type: '%s'", v.Encryption)
	}
	return &v.Value, nil
}

func isNotFoundError(err error) bool {
	_, ok := err.(*backend.NotFoundError)
	return ok
}

const encryptionSecretBox = "secretbox.v1"

func responseToString(r *etcd.Response) string {
	return fmt.Sprintf("%s %s %d", r.Action, r.Node.Key, r.EtcdIndex)
}
