package backend

import (
	"bytes"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	log "github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcan/endpoint"
	"regexp"
	"strings"
)

type EtcdBackend struct {
	nodes       []string
	etcdKey     string
	consistency string
	client      *etcd.Client
	changes     chan *Change
	statsGetter StatsGetter
}

func NewEtcdBackend(nodes []string, etcdKey, consistency string, changes chan *Change, statsGetter StatsGetter) (*EtcdBackend, error) {
	client := etcd.NewClient(nodes)
	if err := client.SetConsistency(consistency); err != nil {
		return nil, err
	}
	b := &EtcdBackend{
		nodes:       nodes,
		etcdKey:     etcdKey,
		consistency: consistency,
		client:      client,
		changes:     changes,
		statsGetter: statsGetter,
	}
	if changes != nil {
		go b.watchChanges()
	}
	return b, nil
}

func (s *EtcdBackend) watchChanges() {
	waitIndex := uint64(0)
	for {
		response, err := s.client.Watch(s.etcdKey, waitIndex, true, nil, nil)
		if err != nil {
			log.Errorf("Failed to get response from etcd: %s, quitting watch goroutine", err)
			return
		}
		log.Infof("Got response: %s %s %d %s",
			response.Action, response.Node.Key, response.EtcdIndex, err)
		change, err := s.parseChange(response)
		if err != nil {
			log.Errorf("Failed to process change: %s, ignoring", err)
			continue
		}
		if change != nil {
			s.changes <- change
		}
		waitIndex = response.Node.ModifiedIndex + 1
	}
}

type MatcherFn func(*etcd.Response) (*Change, error)

func (s *EtcdBackend) parseChange(response *etcd.Response) (*Change, error) {
	matchers := []MatcherFn{
		s.parseHostChange,
		s.parseLocationChange,
		s.parseUpstreamChange,
	}
	for _, matcher := range matchers {
		a, err := matcher(response)
		if a != nil || err != nil {
			return a, err
		}
	}
	return nil, nil
}

func (s *EtcdBackend) parseUpstreamChange(response *etcd.Response) (*Change, error) {
	out := regexp.MustCompile("/upstreams/([^/]+)/endpoints/([^/]+)").FindStringSubmatch(response.Node.Key)
	if len(out) != 3 {
		return nil, nil
	}
	upstreamId, endpointId := out[1], out[2]
	upstream, err := s.readUpstream(upstreamId)
	if err != nil {
		return nil, err
	}
	change := &Change{
		Action: response.Action,
		Parent: upstream,
	}
	if response.Action == "create" {
		for _, e := range upstream.Endpoints {
			if e.Name == endpointId {
				change.Child = e
				return change, nil
			}
		}
		return nil, fmt.Errorf("Endpoint %s not found", endpointId)
	} else if response.Action == "delete" {
		change.Child = &Endpoint{Name: endpointId}
		return change, nil
	}
	return nil, fmt.Errorf("Unsupported action on the endpoint: %s", response.Action)
}

func (s *EtcdBackend) parseHostChange(response *etcd.Response) (*Change, error) {
	out := regexp.MustCompile("/hosts/([^/]+)$").FindStringSubmatch(response.Node.Key)
	if len(out) != 2 {
		return nil, nil
	}
	return &Change{
		Action: response.Action,
		Parent: nil,
		Child:  &Host{Name: out[1]},
	}, nil
}

func (s *EtcdBackend) parseLocationChange(response *etcd.Response) (*Change, error) {
	out := regexp.MustCompile("/hosts/([^/]+)/locations/([^/]+)/upstream").FindStringSubmatch(response.Node.Key)
	if len(out) != 3 {
		return nil, nil
	}
	hostname, locationId := out[1], out[2]
	host, err := s.readHost(hostname)
	if err != nil {
		return nil, err
	}
	change := &Change{
		Action: response.Action,
		Parent: host,
	}
	if response.Action == "create" {
		location, err := s.readLocation(hostname, locationId)
		if err != nil {
			return nil, err
		}
		change.Child = location
		return change, nil
	} else if response.Action == "delete" {
		change.Child = &Location{Name: locationId}
		return change, nil
	} else if response.Action == "set" {
		location, err := s.readLocation(hostname, locationId)
		if err != nil {
			return nil, err
		}
		change.Child = location
		change.Keys = map[string]string{"upstream": response.Node.Value}
		return change, nil
	}
	return nil, fmt.Errorf("Unsupported action on the location: %s", response.Action)
}

func (s *EtcdBackend) GetHosts() ([]*Host, error) {
	return s.readHosts()
}

func (s *EtcdBackend) AddHost(name string) error {
	_, err := s.client.CreateDir(join(s.etcdKey, "hosts", name), 0)
	if isDupe(err) {
		return fmt.Errorf("Host '%s' already exists", name)
	}
	return err
}

func (s *EtcdBackend) DeleteHost(name string) error {
	_, err := s.client.Delete(join(s.etcdKey, "hosts", name), true)
	return err
}

func (s *EtcdBackend) AddLocation(id, hostname, path, upstreamId string) error {
	log.Infof("Add Location(id=%s, hosntame=%s, path=%s, upstream=%s)", id, hostname, path, upstreamId)
	// Make sure location path is a valid regular expression
	if _, err := regexp.Compile(path); err != nil {
		return fmt.Errorf("Path should be a valid Golang regular expression")
	}

	// Make sure upstream actually exists
	_, err := s.readUpstream(upstreamId)
	if err != nil {
		return err
	}
	// Create the location
	if id == "" {
		response, err := s.client.AddChildDir(join(s.etcdKey, "hosts", hostname, "locations"), 0)
		if err != nil {
			return formatErr(err)
		}
		id = suffix(response.Node.Key)
	} else {
		_, err := s.client.CreateDir(join(s.etcdKey, "hosts", hostname, "locations", id), 0)
		if err != nil {
			return formatErr(err)
		}
	}
	locationKey := join(s.etcdKey, "hosts", hostname, "locations", id)
	if _, err := s.client.Create(join(locationKey, "path"), path, 0); err != nil {
		return formatErr(err)
	}
	if _, err := s.client.Create(join(locationKey, "upstream"), upstreamId, 0); err != nil {
		return formatErr(err)
	}
	return nil
}

func (s *EtcdBackend) UpdateLocationUpstream(hostname, id, upstreamId string) error {
	log.Infof("Update Location(id=%s, hostname=%s) set upstream %s", id, hostname, upstreamId)

	// Make sure upstream actually exists
	_, err := s.readUpstream(upstreamId)
	if err != nil {
		return err
	}

	// Make sure location actually exists
	location, err := s.readLocation(hostname, id)
	if err != nil {
		return err
	}

	// Update upstream
	if _, err := s.client.Set(join(location.EtcdKey, "upstream"), upstreamId, 0); err != nil {
		return formatErr(err)
	}

	return nil
}

func (s *EtcdBackend) DeleteLocation(hostname, id string) error {
	locationKey := join(s.etcdKey, "hosts", hostname, "locations", id)
	if _, err := s.client.Delete(locationKey, true); err != nil {
		return formatErr(err)
	}
	return nil
}

func (s *EtcdBackend) GetUpstreams() ([]*Upstream, error) {
	return s.readUpstreams()
}

func (s *EtcdBackend) AddUpstream(upstreamId string) error {
	if upstreamId == "" {
		if _, err := s.client.AddChildDir(join(s.etcdKey, "upstreams"), 0); err != nil {
			return formatErr(err)
		}
	} else {
		if _, err := s.client.CreateDir(join(s.etcdKey, "upstreams", upstreamId), 0); err != nil {
			if isDupe(err) {
				return fmt.Errorf("Upstream '%s' already exists", upstreamId)
			}
			return formatErr(err)
		}
	}
	return nil
}

func (s *EtcdBackend) DeleteUpstream(upstreamId string) error {
	locations, err := s.upstreamUsedBy(upstreamId)
	if err != nil {
		return err
	}
	if len(locations) != 0 {
		return fmt.Errorf("Can't delete upstream '%s', it's in use by %s", locations)
	}
	_, err = s.client.Delete(join(s.etcdKey, "upstreams", upstreamId), true)
	return err
}

func (s *EtcdBackend) AddEndpoint(upstreamId, id, url string) error {
	if _, err := endpoint.ParseUrl(url); err != nil {
		return fmt.Errorf("Endpoint url '%s' is not valid")
	}
	if _, err := s.readUpstream(upstreamId); err != nil {
		return formatErr(err)
	}
	if id == "" {
		if _, err := s.client.AddChild(join(s.etcdKey, "upstreams", upstreamId, "endpoints"), url, 0); err != nil {
			return formatErr(err)
		}
	} else {
		if _, err := s.client.Create(join(s.etcdKey, "upstreams", upstreamId, "endpoints", id), url, 0); err != nil {
			return formatErr(err)
		}
	}
	return nil
}

func (s *EtcdBackend) DeleteEndpoint(upstreamId, id string) error {
	if _, err := s.readUpstream(upstreamId); err != nil {
		if notFound(err) {
			return fmt.Errorf("Upstream '%s' not found", upstreamId)
		}
		return err
	}
	if _, err := s.client.Delete(join(s.etcdKey, "upstreams", upstreamId, "endpoints", id), true); err != nil {
		if notFound(err) {
			return fmt.Errorf("Endpoint '%s' not found", id)
		}
	}
	return nil
}

func (s *EtcdBackend) readHosts() ([]*Host, error) {
	hosts := []*Host{}
	for _, hostKey := range s.getDirs(s.etcdKey, "hosts") {
		host, err := s.readHost(suffix(hostKey))
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func (s *EtcdBackend) readUpstreams() ([]*Upstream, error) {
	upstreams := []*Upstream{}
	for _, upstreamKey := range s.getDirs(s.etcdKey, "upstreams") {
		upstream, err := s.readUpstream(suffix(upstreamKey))
		if err != nil {
			return nil, err
		}
		upstreams = append(upstreams, upstream)
	}
	return upstreams, nil
}

func (s *EtcdBackend) readHost(hostname string) (*Host, error) {
	hostKey := join(s.etcdKey, "hosts", hostname)
	_, err := s.client.Get(hostKey, false, false)
	if err != nil {
		if notFound(err) {
			return nil, fmt.Errorf("Host '%s' not found", hostname)
		}
		return nil, err
	}
	host := &Host{
		Name:      hostname,
		EtcdKey:   hostKey,
		Locations: []*Location{},
	}

	for _, locationKey := range s.getDirs(hostKey, "locations") {
		location, err := s.readLocation(hostname, suffix(locationKey))
		if err != nil {
			return nil, err
		}
		host.Locations = append(host.Locations, location)
	}
	return host, nil
}

func (s *EtcdBackend) readLocation(hostname, locationId string) (*Location, error) {
	locationKey := join(s.etcdKey, "hosts", hostname, "locations", locationId)
	_, err := s.client.Get(locationKey, false, false)
	if err != nil {
		if notFound(err) {
			return nil, fmt.Errorf("Location '%s' not found for Host '%s'", locationId, hostname)
		}
		return nil, err
	}
	path, ok := s.getVal(locationKey, "path")
	if !ok {
		return nil, fmt.Errorf("Missing location path: %s", locationKey)
	}
	upstreamKey, ok := s.getVal(locationKey, "upstream")
	if !ok {
		return nil, fmt.Errorf("Missing location upstream: %s", locationKey)
	}
	location := &Location{
		Name:    suffix(locationKey),
		EtcdKey: locationKey,
		Path:    path,
	}
	upstream, err := s.readUpstream(upstreamKey)
	if err != nil {
		return nil, err
	}
	for _, e := range upstream.Endpoints {
		stats, err := s.statsGetter.GetStats(hostname, locationId, e.Name)
		if err == nil {
			e.Stats = stats
		} else {
			log.Errorf("Failed to get stats about endpoint: %s, err: %s", e, err)
		}
	}
	location.Upstream = upstream
	return location, nil
}

func (s *EtcdBackend) readUpstream(upstreamId string) (*Upstream, error) {
	upstreamKey := join(s.etcdKey, "upstreams", upstreamId)

	_, err := s.client.Get(upstreamKey, false, false)
	if err != nil {
		return nil, err
	}
	upstream := &Upstream{
		Name:      suffix(upstreamKey),
		EtcdKey:   upstreamKey,
		Endpoints: []*Endpoint{},
	}

	endpointPairs := s.getVals(join(upstream.EtcdKey, "endpoints"))
	for _, e := range endpointPairs {
		_, err := endpoint.ParseUrl(e.Val)
		if err != nil {
			fmt.Printf("Ignoring endpoint: failed to parse url: %s", e.Val)
			continue
		}
		e := &Endpoint{
			Url:     e.Val,
			EtcdKey: e.Key,
			Name:    suffix(e.Key),
		}
		upstream.Endpoints = append(upstream.Endpoints, e)
	}
	return upstream, nil
}

func (s *EtcdBackend) findLocation(hostname, path string) (*Location, error) {
	hosts, err := s.readHosts()
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		if h.Name == hostname {
			for _, l := range h.Locations {
				if l.Path == path {
					return l, nil
				}
			}
		}
	}
	return nil, nil
}

func (s *EtcdBackend) upstreamUsedBy(upstreamId string) ([]*Location, error) {
	var locations []*Location
	hosts, err := s.readHosts()
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		for _, l := range h.Locations {
			if l.Upstream.Name == upstreamId {
				locations = append(locations, l)
			}
		}
	}
	return locations, nil
}

func (s *EtcdBackend) getVal(keys ...string) (string, bool) {
	response, err := s.client.Get(strings.Join(keys, "/"), false, false)
	if notFound(err) {
		return "", false
	}
	if isDir(response.Node) {
		return "", false
	}
	return response.Node.Value, true
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

func (s *EtcdBackend) getVals(keys ...string) []Pair {
	var out []Pair
	response, err := s.client.Get(strings.Join(keys, "/"), true, true)
	if notFound(err) {
		return out
	}

	if !isDir(response.Node) {
		return out
	}

	for _, srvNode := range response.Node.Nodes {
		if !isDir(&srvNode) {
			out = append(out, Pair{srvNode.Key, srvNode.Value})
		}
	}
	return out
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

func formatErr(e error) error {
	switch err := e.(type) {
	case *etcd.EtcdError:
		return fmt.Errorf("Key error: %s", err.Message)
	}
	return e
}

func notFound(err error) bool {
	if err == nil {
		return false
	}
	eErr, ok := err.(*etcd.EtcdError)
	return ok && eErr.ErrorCode == 100
}

func isDupe(err error) bool {
	if err == nil {
		return false
	}
	eErr, ok := err.(*etcd.EtcdError)
	return ok && eErr.ErrorCode == 105
}

func isDir(n *etcd.Node) bool {
	return n != nil && n.Dir == true
}

func enumerateLocations(ls []*Location) string {
	b := &bytes.Buffer{}
	for i, l := range ls {
		b.WriteString(l.Name)
		if i != len(ls)-1 {
			b.WriteString(", ")
		}
	}
	return b.String()
}
