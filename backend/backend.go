// Package backend defines interfaces and structures controlling the proxy configuration and changes.
package backend

import (
	"crypto/subtle"
	"crypto/tls"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/failover"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/plugin"
)

type NewBackendFn func() (Backend, error)

type Backend interface {
	GetHosts() ([]*Host, error)
	AddHost(*Host) (*Host, error)
	DeleteHost(name string) error
	UpdateHostKeyPair(hostname string, keyPair *KeyPair) (*Host, error)

	GetHost(name string) (*Host, error)

	AddHostListener(hostname string, listener *Listener) (*Listener, error)
	DeleteHostListener(hostname string, listenerId string) error

	AddLocation(*Location) (*Location, error)
	GetLocation(hostname, id string) (*Location, error)
	UpdateLocationUpstream(hostname, id string, upstream string) (*Location, error)
	UpdateLocationOptions(hostname, locationId string, o LocationOptions) (*Location, error)
	DeleteLocation(hostname, id string) error

	AddLocationMiddleware(hostname, locationId string, m *MiddlewareInstance) (*MiddlewareInstance, error)
	GetLocationMiddleware(hostname, locationId string, mType, id string) (*MiddlewareInstance, error)
	UpdateLocationMiddleware(hostname, locationId string, m *MiddlewareInstance) (*MiddlewareInstance, error)
	DeleteLocationMiddleware(hostname, locationId, mType, id string) error

	GetUpstreams() ([]*Upstream, error)
	AddUpstream(*Upstream) (*Upstream, error)
	GetUpstream(id string) (*Upstream, error)
	DeleteUpstream(id string) error

	AddEndpoint(*Endpoint) (*Endpoint, error)
	GetEndpoint(upstreamId, id string) (*Endpoint, error)
	DeleteEndpoint(upstreamId, id string) error

	// WatchChanges is an entry point for getting the configuration changes as well as the initial configuration.
	// It should be a blocking function generating events from change.go to the changes channel.
	WatchChanges(changes chan interface{}, cancel chan bool) error

	// GetRegistry returns registry with the supported plugins.
	GetRegistry() *plugin.Registry

	Close()
}

// StatsGetter provides realtime stats abount endpoints, upstreams and locations
type StatsGetter interface {
	GetLocationStats(l *Location) (*RoundTripStats, error)
	GetEndpointStats(e *Endpoint) (*RoundTripStats, error)
	GetUpstreamStats(u *Upstream) (*RoundTripStats, error)
}

type KeyPair struct {
	Key  []byte
	Cert []byte
}

func NewKeyPair(cert, key []byte) (*KeyPair, error) {
	if len(cert) == 0 || len(key) == 0 {
		return nil, fmt.Errorf("Provide non-empty certificate and a private key")
	}
	if _, err := tls.X509KeyPair(cert, key); err != nil {
		return nil, err
	}
	return &KeyPair{Cert: cert, Key: key}, nil
}

func (c *KeyPair) Equals(o *KeyPair) bool {
	return (len(c.Cert) == len(o.Cert)) &&
		(len(c.Key) == len(o.Key)) &&
		subtle.ConstantTimeCompare(c.Cert, o.Cert) == 1 &&
		subtle.ConstantTimeCompare(c.Key, o.Key) == 1
}

type Address struct {
	Network string
	Address string
}

// Listener specifies the listening point - the network and interface for each host. Host can have multiple interfaces.
type Listener struct {
	Id string
	// HTTP or HTTPS
	Protocol string
	// Adddress specifies network (tcp or unix) and address (ip:port or path to unix socket)
	Address Address
}

func (l *Listener) String() string {
	return fmt.Sprintf("Listener(%s, %s://%s)", l.Protocol, l.Address.Network, l.Address.Address)
}

func (a *Address) Equals(o Address) bool {
	return a.Network == o.Network && a.Address == o.Address
}

type HostOptions struct {
	Default bool
}

// Incoming requests are matched by their hostname first. Hostname is defined by incoming 'Host' header.
// E.g. curl http://example.com/alice will be matched by the host example.com first.
type Host struct {
	Name      string
	Locations []*Location
	KeyPair   *KeyPair
	Listeners []*Listener
	Options   HostOptions
}

func NewHost(name string) (*Host, error) {
	if name == "" {
		return nil, fmt.Errorf("Hostname can not be empty")
	}
	return &Host{
		Name:      name,
		Locations: []*Location{},
	}, nil
}

func (h *Host) String() string {
	return fmt.Sprintf("Host(%s)", h.Name)
}

func (h *Host) GetId() string {
	return h.Name
}

// Hosts contain one or several locations. Each location defines a path - simply a regular expression that will be matched against request's url.
// Location contains link to an upstream and vulcand will use the endpoints from this upstream to serve the request.
// E.g. location loc1 will serve the request curl http://example.com/alice because it matches the path /alice:
type Location struct {
	Hostname    string
	Path        string
	Id          string
	Upstream    *Upstream
	Middlewares []*MiddlewareInstance
	Options     LocationOptions
	// Combined stats from all endpoints in the location
	Stats RoundTripStats
}

type LocationTimeouts struct {
	// Socket read timeout (before we receive the first reply header)
	Read string
	// Socket connect timeout
	Dial string
	// TLS handshake timeout
	TlsHandshake string
}

type LocationKeepAlive struct {
	// Keepalive period
	Period string
	// How many idle connections will be kept per host
	MaxIdleConnsPerHost int
}

// Limits contains various limits one can supply for a location.
type LocationLimits struct {
	MaxMemBodyBytes int64 // Maximum size to keep in memory before buffering to disk
	MaxBodyBytes    int64 // Maximum size of a request body in bytes
}

// Additional options to control this location, such as timeouts
type LocationOptions struct {
	Timeouts LocationTimeouts
	// Controls KeepAlive settins for backend servers
	KeepAlive LocationKeepAlive
	// Limits contains various limits one can supply for a location.
	Limits LocationLimits
	// Predicate that defines when requests are allowed to failover
	FailoverPredicate string
	// Used in forwarding headers
	Hostname string
	// In this case appends new forward info to the existing header
	TrustForwardHeader bool
}

// Wrapper that contains information about this middleware backend-specific data used for serialization/deserialization
type MiddlewareInstance struct {
	Id         string
	Priority   int
	Type       string
	Middleware plugin.Middleware
}

func NewAddress(network, address string) (*Address, error) {
	if len(address) == 0 {
		return nil, fmt.Errorf("supply a non empty address")
	}

	network = strings.ToLower(network)
	if network != TCP && network != UNIX {
		return nil, fmt.Errorf("unsupported network '%s', supported networks are tcp and unix", network)
	}

	return &Address{Network: network, Address: address}, nil
}

func NewListener(id, protocol, network, address string) (*Listener, error) {
	protocol = strings.ToLower(protocol)
	if protocol != HTTP && protocol != HTTPS {
		return nil, fmt.Errorf("unsupported protocol '%s', supported protocols are http and https", protocol)
	}

	a, err := NewAddress(network, address)
	if err != nil {
		return nil, err
	}

	return &Listener{
		Id:       id,
		Address:  *a,
		Protocol: protocol,
	}, nil
}

func NewLocation(hostname, id, path, upstreamId string) (*Location, error) {
	return NewLocationWithOptions(hostname, id, path, upstreamId, LocationOptions{})
}

func NewLocationWithOptions(hostname, id, path, upstreamId string, options LocationOptions) (*Location, error) {
	if len(path) == 0 || len(hostname) == 0 || len(upstreamId) == 0 {
		return nil, fmt.Errorf("supply valid hostname, path and upstream id")
	}

	// Make sure location path is a valid regular expression
	if _, err := regexp.Compile(path); err != nil {
		return nil, fmt.Errorf("path should be a valid Golang regular expression")
	}

	if _, err := parseLocationOptions(options); err != nil {
		return nil, err
	}

	return &Location{
		Hostname:    hostname,
		Path:        path,
		Id:          id,
		Upstream:    &Upstream{Id: upstreamId, Endpoints: []*Endpoint{}},
		Middlewares: []*MiddlewareInstance{},
		Options:     options,
	}, nil
}

func parseLocationOptions(l LocationOptions) (*httploc.Options, error) {
	o := &httploc.Options{}
	var err error

	// Connection timeouts
	if len(l.Timeouts.Read) != 0 {
		if o.Timeouts.Read, err = time.ParseDuration(l.Timeouts.Read); err != nil {
			return nil, fmt.Errorf("invalid read timeout: %s", err)
		}
	}
	if len(l.Timeouts.Dial) != 0 {
		if o.Timeouts.Dial, err = time.ParseDuration(l.Timeouts.Dial); err != nil {
			return nil, fmt.Errorf("invalid dial timeout: %s", err)
		}
	}
	if len(l.Timeouts.TlsHandshake) != 0 {
		if o.Timeouts.TlsHandshake, err = time.ParseDuration(l.Timeouts.TlsHandshake); err != nil {
			return nil, fmt.Errorf("invalid tls handshake timeout: %s", err)
		}
	}

	// Keep Alive parameters
	if len(l.KeepAlive.Period) != 0 {
		if o.KeepAlive.Period, err = time.ParseDuration(l.KeepAlive.Period); err != nil {
			return nil, fmt.Errorf("invalid tls handshake timeout: %s", err)
		}
	}
	o.KeepAlive.MaxIdleConnsPerHost = l.KeepAlive.MaxIdleConnsPerHost

	// Location-specific limits
	o.Limits.MaxMemBodyBytes = l.Limits.MaxMemBodyBytes
	o.Limits.MaxBodyBytes = l.Limits.MaxBodyBytes

	// Failover predicate
	if len(l.FailoverPredicate) != 0 {
		if o.ShouldFailover, err = failover.ParseExpression(l.FailoverPredicate); err != nil {
			return nil, err
		}
	}

	o.Hostname = l.Hostname
	o.TrustForwardHeader = l.TrustForwardHeader
	return o, nil
}

func (l *Location) GetOptions() (*httploc.Options, error) {
	return parseLocationOptions(l.Options)
}

func (l *Location) String() string {
	return fmt.Sprintf("Location(%s/%s, %s, %s)", l.Hostname, l.Id, l.Path, l.Upstream)
}

func (l *Location) GetId() string {
	return l.Id
}

func (l *Location) GetUniqueId() LocationKey {
	return LocationKey{Hostname: l.Hostname, Id: l.Id}
}

// Upstream is a collection of endpoints. Each location is assigned an upstream. Changing assigned upstream
// of the location gracefully redirects the traffic to the new endpoints of the upstream.
type Upstream struct {
	Id        string
	Endpoints []*Endpoint
}

func NewUpstream(id string) (*Upstream, error) {
	return &Upstream{
		Id:        id,
		Endpoints: []*Endpoint{},
	}, nil
}

func (u *Upstream) String() string {
	return fmt.Sprintf("Upstream(id=%s)", u.Id)
}

func (u *Upstream) GetId() string {
	return u.Id
}

func (u *Upstream) GetUniqueId() UpstreamKey {
	return UpstreamKey{Id: u.Id}
}

// Endpoint is a final destination of the request
type Endpoint struct {
	Id         string
	Url        string
	UpstreamId string
	Stats      RoundTripStats
}

func NewEndpoint(upstreamId, id, url string) (*Endpoint, error) {
	if upstreamId == "" {
		return nil, fmt.Errorf("upstream id can not be empty")
	}
	if _, err := netutils.ParseUrl(url); err != nil {
		return nil, fmt.Errorf("endpoint url '%s' is not valid", url)
	}
	return &Endpoint{
		UpstreamId: upstreamId,
		Id:         id,
		Url:        url,
	}, nil
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("Endpoint(%s, %s, %s, %s)", e.Id, e.UpstreamId, e.Url, e.Stats)
}

func (e *Endpoint) GetId() string {
	return e.Id
}

func (e *Endpoint) GetUniqueId() EndpointKey {
	return EndpointKey{UpstreamId: e.UpstreamId, Id: e.Id}
}

// RoundTrip stats contain real time statistics about performance of Endpoint or Location
// such as latency, processed and failed requests
type RoundTripStats struct {
	Counters Counters
	Latency  Brackets
}

func (e *RoundTripStats) NetErrorRate() float64 {
	if e.Counters.Total == 0 {
		return 0
	}
	return (float64(e.Counters.NetErrors) / float64(e.Counters.Total)) * 100
}

func (e *RoundTripStats) RequestsPerSecond() float64 {
	if e.Counters.Period == 0 {
		return 0
	}
	return float64(e.Counters.Total) / float64(e.Counters.Period/time.Second)
}

func (e *RoundTripStats) String() string {
	return fmt.Sprintf("%.2f requests/sec, %.2f failures/sec", e.RequestsPerSecond(), e.NetErrorRate())
}

type NotFoundError struct {
	Message string
}

func (n *NotFoundError) Error() string {
	if n.Message != "" {
		return n.Message
	} else {
		return "Object not found"
	}
}

type AlreadyExistsError struct {
	Message string
}

func (n *AlreadyExistsError) Error() string {
	return n.Message
}

type Counters struct {
	Period      time.Duration
	NetErrors   int64
	Total       int64
	StatusCodes []StatusCode
}

type StatusCode struct {
	Code  int
	Count int64
}

type Brackets struct {
	Q50 int64
	Q75 int64
	Q95 int64
	Q99 int64
}

func NewBrackets(h metrics.Histogram) Brackets {
	return Brackets{
		Q50: h.ValueAtQuantile(50),
		Q75: h.ValueAtQuantile(75),
		Q95: h.ValueAtQuantile(95),
		Q99: h.ValueAtQuantile(99),
	}
}

type LocationKey struct {
	Hostname string
	Id       string
}

func (l LocationKey) String() string {
	return fmt.Sprintf("%s.%s", l.Hostname, l.Id)
}

type EndpointKey struct {
	UpstreamId string
	Id         string
}

func (e EndpointKey) String() string {
	return fmt.Sprintf("%s.%s", e.UpstreamId, e.Id)
}

func ParseEndpointKey(v string) (*EndpointKey, error) {
	out := strings.SplitN(v, ".", 2)
	if len(out) != 2 {
		return nil, fmt.Errorf("invalid id: '%s'", v)
	}
	return &EndpointKey{UpstreamId: out[0], Id: out[1]}, nil
}

func MustParseEndpointKey(v string) EndpointKey {
	k, err := ParseEndpointKey(v)
	if err != nil {
		panic(err)
	}
	return *k
}

type UpstreamKey struct {
	Id string
}

func (u UpstreamKey) String() string {
	return u.Id
}

const (
	HTTP  = "http"
	HTTPS = "https"
	TCP   = "tcp"
	UNIX  = "unix"
)
