package scroll

import (
	"fmt"
	"strings"

	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
)

const (
	endpointKey = "vulcand/upstreams/%v/endpoints/%v"
	locationKey = "vulcand/hosts/%v/locations/%v"

	// If vulcand registration is enabled, the app will be re-registering itself every
	// this amount of seconds.
	endpointTTL = 60 // seconds
)

type registry struct {
	etcdClient *etcd.Client
}

type endpoint struct {
	upstream string
	host     string
	port     int
}

type location struct {
	apiHost  string
	methods  []string
	path     string
	upstream string
}

func newRegistry() *registry {
	return &registry{
		etcdClient: etcd.NewClient([]string{"http://127.0.0.1:4001"}),
	}
}

func (r *registry) RegisterEndpoint(e *endpoint) error {
	key := fmt.Sprintf(endpointKey, e.GetUpstream(), e.GetID())

	if _, err := r.etcdClient.Set(key, e.GetEndpoint(), endpointTTL); err != nil {
		return err
	}

	return nil
}

func (r *registry) RegisterLocation(l *location) error {
	key := fmt.Sprintf(locationKey, l.GetAPIHost(), l.GetID())

	pathKey := fmt.Sprintf("%v/path", key)
	if _, err := r.etcdClient.Set(pathKey, l.GetPath(), 0); err != nil {
		return err
	}

	upstreamKey := fmt.Sprintf("%v/upstream", key)
	if _, err := r.etcdClient.Set(upstreamKey, l.GetUpstream(), 0); err != nil {
		return err
	}

	return nil
}

func newEndpoint(upstream, host string, port int) *endpoint {
	return &endpoint{
		upstream: upstream,
		host:     host,
		port:     port,
	}
}

func (e *endpoint) GetID() string {
	return fmt.Sprintf("%v_%v", e.host, e.port)
}

func (e *endpoint) GetUpstream() string {
	return e.upstream
}

func (e *endpoint) GetEndpoint() string {
	return fmt.Sprintf("http://%v:%v", e.host, e.port)
}

func (e *endpoint) String() string {
	return fmt.Sprintf("id [%v], upstream [%v], endpoint [%v]",
		e.GetID(), e.GetUpstream(), e.GetEndpoint())
}

func newLocation(apiHost string, methods []string, path, upstream string) *location {
	return &location{
		apiHost:  apiHost,
		methods:  methods,
		path:     convertPath(path),
		upstream: upstream,
	}
}

func (l *location) GetID() string {
	return strings.Replace(fmt.Sprintf("%v%v", strings.Join(l.methods, "."), l.path), "/", ".", -1)
}

func (l *location) GetAPIHost() string {
	return l.apiHost
}

func (l *location) GetPath() string {
	methods := strings.Join(l.methods, `", "`)
	return fmt.Sprintf(`TrieRoute("%v", "%v")`, methods, l.path)
}

func (l *location) GetUpstream() string {
	return l.upstream
}

func (l *location) String() string {
	return fmt.Sprintf("id [%v], API host [%v], path [%v], upstream [%v]",
		l.GetID(), l.GetAPIHost(), l.GetPath(), l.GetUpstream())
}

// Convert router path to the format understood by vulcand.
//
// Effectively, just replaces curly brackets with angle brackets.
func convertPath(path string) string {
	return strings.Replace(strings.Replace(path, "{", "<", -1), "}", ">", -1)
}
