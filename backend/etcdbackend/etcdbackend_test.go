// Note on debugging:
// github.com/davecgh/go-spew/spew package is extremely helpful when it comes to debugging DeepEquals issues.
// Here's how one uses it:
// spew.Printf("%#v\n vs\n %#v\n", a, b)
//
package etcdbackend

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"github.com/mailgun/vulcand/secret"

	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin/ratelimit"
	. "github.com/mailgun/vulcand/plugin/registry"
)

func TestEtcdBackend(t *testing.T) { TestingT(t) }

type EtcdBackendSuite struct {
	backend      *EtcdBackend
	nodes        []string
	etcdPrefix   string
	consistency  string
	client       *etcd.Client
	changesC     chan interface{}
	timeProvider *timetools.FreezedTime
	key          string
	stopC        chan bool
}

var _ = Suite(&EtcdBackendSuite{
	etcdPrefix:  "/vulcandtest",
	consistency: etcd.STRONG_CONSISTENCY,
	timeProvider: &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	},
})

func (s *EtcdBackendSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	key, err := secret.NewKeyString()
	if err != nil {
		panic(err)
	}
	s.key = key

	nodes_string := os.Getenv("VULCAND_TEST_ETCD_NODES")
	if nodes_string == "" {
		// Skips the entire suite
		c.Skip("This test requires etcd, provide comma separated nodes in VULCAND_TEST_ETCD_NODES environment variable")
		return
	}

	s.nodes = strings.Split(nodes_string, ",")
}

func (s *EtcdBackendSuite) SetUpTest(c *C) {
	// Initiate a backend with a registry

	key, err := secret.KeyFromString(s.key)
	c.Assert(err, IsNil)

	box, err := secret.NewBox(key)
	c.Assert(err, IsNil)

	backend, err := NewEtcdBackendWithOptions(
		GetRegistry(),
		s.nodes,
		s.etcdPrefix,
		Options{
			EtcdConsistency: s.consistency,
			Box:             box,
		})
	c.Assert(err, IsNil)
	s.backend = backend
	s.client = s.backend.client

	// Delete all values under the given prefix
	_, err = s.client.Get(s.etcdPrefix, false, false)
	if err != nil {
		// There's no key like this
		if !notFound(err) {
			// We haven't expected this error, oops
			c.Assert(err, IsNil)
		}
	} else {
		_, err = s.backend.client.Delete(s.etcdPrefix, true)
		c.Assert(err, IsNil)
	}

	s.changesC = make(chan interface{})
	s.stopC = make(chan bool)
	go s.backend.WatchChanges(s.changesC, s.stopC)
}

func (s *EtcdBackendSuite) TearDownTest(c *C) {
	close(s.stopC)
	s.backend.Close()
}

func (s *EtcdBackendSuite) collectChanges(c *C, expected int) []interface{} {
	changes := make([]interface{}, expected)
	for i, _ := range changes {
		select {
		case changes[i] = <-s.changesC:
			// successfully collected changes
		case <-time.After(2 * time.Second):
			c.Fatalf("Timeout occured")
		}
	}
	return changes
}

func (s *EtcdBackendSuite) expectChanges(c *C, expected ...interface{}) {
	changes := s.collectChanges(c, len(expected))
	for i, ch := range changes {
		c.Assert(ch, DeepEquals, expected[i])
	}
}

func (s *EtcdBackendSuite) TestAddDeleteHost(c *C) {
	host := s.makeHost("localhost")

	h, err := s.backend.AddHost(host)
	c.Assert(err, IsNil)
	c.Assert(h, Equals, host)

	s.expectChanges(c, &HostAdded{Host: host})

	err = s.backend.DeleteHost("localhost")
	c.Assert(err, IsNil)

	s.expectChanges(c, &HostDeleted{
		Name: "localhost",
	})
}

func (s *EtcdBackendSuite) TestAddExpireHost(c *C) {
	host := s.makeHost("localhost")

	_, err := s.client.SetDir(s.backend.path("hosts", host.Name), 1)
	c.Assert(err, IsNil)

	s.expectChanges(c, &HostAdded{Host: host}, &HostDeleted{Name: host.Name})
}

func (s *EtcdBackendSuite) TestAddHostWithOptions(c *C) {
	host := s.makeHost("localhost")
	host.Options.Default = true

	h, err := s.backend.AddHost(host)
	c.Assert(err, IsNil)
	c.Assert(h, Equals, host)

	s.expectChanges(c, &HostAdded{Host: host})

	err = s.backend.DeleteHost("localhost")
	c.Assert(err, IsNil)

	s.expectChanges(c, &HostDeleted{
		Name: "localhost",
	})
}

func (s *EtcdBackendSuite) TestAddHostWithKeyPair(c *C) {
	host := s.makeHost("localhost")
	host.KeyPair = &KeyPair{
		Key:  []byte("hello"),
		Cert: []byte("world"),
	}

	h, err := s.backend.AddHost(host)
	c.Assert(err, IsNil)
	c.Assert(h, Equals, host)

	hostNoKeyPair := *host
	hostNoKeyPair.KeyPair = nil

	s.expectChanges(c, &HostAdded{Host: &hostNoKeyPair}, &HostKeyPairUpdated{Host: host})
}

func (s *EtcdBackendSuite) TestAddHostWithListeners(c *C) {
	host := s.makeHost("localhost")
	host.Listeners = []*Listener{
		&Listener{
			Protocol: "http",
			Address: Address{
				Network: "tcp",
				Address: "127.0.0.1:9000",
			},
		},
	}

	h, err := s.backend.AddHost(host)
	c.Assert(err, IsNil)
	c.Assert(h, Equals, host)

	hostNoListeners := *host
	hostNoListeners.Listeners = []*Listener{}

	s.expectChanges(c, &HostAdded{Host: &hostNoListeners}, &HostListenerAdded{Host: host, Listener: host.Listeners[0]})
}

func (s *EtcdBackendSuite) TestAddHostListener(c *C) {
	host := s.makeHost("localhost")

	h, err := s.backend.AddHost(host)
	c.Assert(err, IsNil)
	c.Assert(h, Equals, host)

	host.Listeners = []*Listener{
		&Listener{
			Id:       "ls1",
			Protocol: "http",
			Address: Address{
				Network: "tcp",
				Address: "127.0.0.1:9000",
			},
		},
	}

	_, err = s.backend.AddHostListener(host.Name, host.Listeners[0])
	c.Assert(err, IsNil)

	hostNoListeners := *host
	hostNoListeners.Listeners = []*Listener{}

	s.expectChanges(c, &HostAdded{Host: &hostNoListeners}, &HostListenerAdded{Host: host, Listener: host.Listeners[0]})

	// Adding same address second time fails
	_, err = s.backend.AddHostListener(host.Name, host.Listeners[0])
	c.Assert(err, NotNil)

	c.Assert(s.backend.DeleteHostListener(host.Name, host.Listeners[0].Id), IsNil)
	s.expectChanges(c, &HostListenerDeleted{Host: &hostNoListeners, ListenerId: host.Listeners[0].Id})
}

func (s *EtcdBackendSuite) TestUpdateHostKeyPair(c *C) {
	host := s.makeHost("localhost")

	h, err := s.backend.AddHost(host)
	c.Assert(err, IsNil)
	c.Assert(h, Equals, host)

	hostNoKeyPair := *host
	hostNoKeyPair.KeyPair = nil

	host.KeyPair = &KeyPair{
		Key:  []byte("hello"),
		Cert: []byte("world"),
	}
	s.backend.UpdateHostKeyPair(host.Name, host.KeyPair)

	s.expectChanges(c, &HostAdded{Host: &hostNoKeyPair}, &HostKeyPairUpdated{Host: host})
}

func (s *EtcdBackendSuite) TestGetUpstreams(c *C) {
	up := s.makeUpstream("u1", 1)
	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	_, err = s.backend.AddEndpoint(up.Endpoints[0])
	c.Assert(err, IsNil)

	upstreams, err := s.backend.GetUpstreams()
	c.Assert(err, IsNil)
	c.Assert(len(upstreams), Equals, 1)
	c.Assert(upstreams[0], DeepEquals, up)
}

// Add the host twice fails
func (s *EtcdBackendSuite) TestAddTwice(c *C) {

	_, err := s.backend.AddHost(&Host{Name: "localhost"})
	c.Assert(err, IsNil)

	_, err = s.backend.AddHost(&Host{Name: "localhost"})
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
}

func (s *EtcdBackendSuite) TestUpstreamCRUD(c *C) {
	up := s.makeUpstream("up1", 0)
	u, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, up)

	s.expectChanges(c, &UpstreamAdded{Upstream: up}, &UpstreamOptionsUpdated{Upstream: up})

	upR, err := s.backend.GetUpstream("up1")
	c.Assert(err, IsNil)
	c.Assert(upR, NotNil)
	c.Assert(upR.Id, Equals, "up1")

	o := UpstreamOptions{Timeouts: UpstreamTimeouts{Read: "1s"}}
	out, err := s.backend.UpdateUpstreamOptions(up.Id, o)
	c.Assert(err, IsNil)
	c.Assert(out.Options, DeepEquals, o)

	s.expectChanges(c, &UpstreamOptionsUpdated{Upstream: out})

	err = s.backend.DeleteUpstream("up1")
	c.Assert(err, IsNil)

	s.expectChanges(c, &UpstreamDeleted{
		UpstreamId: "up1",
	})
}

func (s *EtcdBackendSuite) TestUpstreamAutoId(c *C) {
	u, err := s.backend.AddUpstream(&Upstream{Endpoints: []*Endpoint{}})

	c.Assert(err, IsNil)
	c.Assert(u, NotNil)
	s.expectChanges(c, &UpstreamAdded{Upstream: u})
}

func (s *EtcdBackendSuite) TestUpstreamTwice(c *C) {
	_, err := s.backend.AddUpstream(&Upstream{Id: "up1"})
	c.Assert(err, IsNil)

	_, err = s.backend.AddUpstream(&Upstream{Id: "up1"})
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
}

func (s *EtcdBackendSuite) TestEndpointAddReadDelete(c *C) {
	up0 := s.makeUpstream("up1", 0)

	_, err := s.backend.AddUpstream(up0)
	c.Assert(err, IsNil)

	s.expectChanges(c,
		&UpstreamAdded{Upstream: up0},
		&UpstreamOptionsUpdated{Upstream: up0})

	up := s.makeUpstream("up1", 1)
	e := up.Endpoints[0]

	eR, err := s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)
	c.Assert(eR, Equals, e)

	eO, err := s.backend.GetEndpoint(e.UpstreamId, e.Id)
	c.Assert(err, IsNil)
	c.Assert(eO, DeepEquals, e)

	s.expectChanges(c, &EndpointUpdated{
		Upstream: up,
		Endpoint: e,
	})

	err = s.backend.DeleteEndpoint(up.Id, e.Id)
	c.Assert(err, IsNil)

	s.expectChanges(c, &EndpointDeleted{
		Upstream:   up0,
		EndpointId: e.Id,
	})
}

func (s *EtcdBackendSuite) TestAddEndpointUsingSet(c *C) {
	up := s.makeUpstream("u1", 1)
	e := up.Endpoints[0]

	_, err := s.client.Set(s.backend.path("upstreams", up.Id, "endpoints", e.Id), e.Url, 0)
	c.Assert(err, IsNil)

	s.expectChanges(c, &EndpointUpdated{
		Upstream: up,
		Endpoint: up.Endpoints[0],
	})
}

func (s *EtcdBackendSuite) TestExpireEndpoint(c *C) {
	up := s.makeUpstream("u1", 1)
	e := up.Endpoints[0]

	_, err := s.client.Set(s.backend.path("upstreams", up.Id, "endpoints", e.Id), e.Url, 1)
	c.Assert(err, IsNil)

	s.expectChanges(c, &EndpointUpdated{
		Upstream: up,
		Endpoint: up.Endpoints[0],
	}, &EndpointDeleted{
		Upstream:   s.makeUpstream(up.Id, 0),
		EndpointId: e.Id,
	})
}

func (s *EtcdBackendSuite) TestAddEndpointAutoId(c *C) {
	up := s.makeUpstream("up1", 1)
	e := up.Endpoints[0]
	e.Id = ""

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)
	eR, err := s.backend.AddEndpoint(e)
	c.Assert(len(eR.Id), Not(Equals), 0)
}

func (s *EtcdBackendSuite) TestDeleteBadEndpoint(c *C) {
	up := s.makeUpstream("up1", 1)

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	// Non existent endpoint
	c.Assert(s.backend.DeleteEndpoint(up.Id, "notHere"), FitsTypeOf, &NotFoundError{})
	// Non existent upstream
	c.Assert(s.backend.DeleteEndpoint("upNotHere", "notHere"), FitsTypeOf, &NotFoundError{})
}

func (s *EtcdBackendSuite) TestLocationAddReadDelete(c *C) {
	up := s.makeUpstream("u1", 1)
	e := up.Endpoints[0]

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)

	host := s.makeHost("localhost")

	_, err = s.backend.AddHost(host)
	c.Assert(err, IsNil)
	s.collectChanges(c, 4)

	loc := s.makeLocation("loc1", "/hello", host, up)

	// CREATE
	locR, err := s.backend.AddLocation(loc)
	c.Assert(err, IsNil)
	c.Assert(locR, DeepEquals, loc)

	// READ
	locR2, err := s.backend.GetLocation(loc.Hostname, loc.Id)
	c.Assert(err, IsNil)
	c.Assert(locR2, DeepEquals, loc)

	s.expectChanges(c, &LocationUpstreamUpdated{
		Host:     host,
		Location: loc,
	})

	// DELETE
	c.Assert(s.backend.DeleteLocation(loc.Hostname, loc.Id), IsNil)
	s.expectChanges(c, &LocationDeleted{
		Host:       host,
		LocationId: loc.Id,
	})
}

func (s *EtcdBackendSuite) TestLocationAddWithOptions(c *C) {
	up := s.makeUpstream("u1", 1)
	e := up.Endpoints[0]

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)

	host := s.makeHost("localhost")

	_, err = s.backend.AddHost(host)
	c.Assert(err, IsNil)
	s.collectChanges(c, 3)

	loc := s.makeLocationWithOptions("loc1", "/hello", host, up, LocationOptions{Hostname: "host1"})

	// CREATE
	locR, err := s.backend.AddLocation(loc)
	c.Assert(err, IsNil)
	c.Assert(locR, DeepEquals, loc)

	// READ
	locR2, err := s.backend.GetLocation(loc.Hostname, loc.Id)
	c.Assert(err, IsNil)
	c.Assert(locR2, DeepEquals, loc)
}

// Make sure we can generate location id when it's not supplied
func (s *EtcdBackendSuite) TestLocationAutoId(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)

	_, err = s.backend.AddHost(host)
	c.Assert(err, IsNil)
	s.collectChanges(c, 3)

	locR, err := s.backend.AddLocation(s.makeLocation("", "/hello", host, up))
	c.Assert(err, IsNil)
	c.Assert(len(locR.Id), Not(Equals), 0)
}

func (s *EtcdBackendSuite) TestLocationUpdateUpstream(c *C) {
	up1 := s.makeUpstream("u1", 1)
	up2 := s.makeUpstream("u2", 1)

	host := s.makeHost("localhost")

	_, err := s.backend.AddUpstream(up1)
	c.Assert(err, IsNil)
	_, err = s.backend.AddEndpoint(up1.Endpoints[0])
	c.Assert(err, IsNil)

	_, err = s.backend.AddUpstream(up2)
	c.Assert(err, IsNil)
	_, err = s.backend.AddEndpoint(up2.Endpoints[0])
	c.Assert(err, IsNil)

	_, err = s.backend.AddHost(host)
	c.Assert(err, IsNil)
	s.collectChanges(c, 7)

	loc := s.makeLocation("loc1", "/hello", host, up1)

	_, err = s.backend.AddLocation(loc)
	c.Assert(err, IsNil)
	s.collectChanges(c, 1)

	locU, err := s.backend.UpdateLocationUpstream(loc.Hostname, loc.Id, up2.Id)
	c.Assert(err, IsNil)
	c.Assert(locU.Upstream, DeepEquals, up2)

	s.expectChanges(c, &LocationUpstreamUpdated{
		Host:     host,
		Location: locU,
	})
}

func (s *EtcdBackendSuite) TestLocationUpdateOptions(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)
	_, err = s.backend.AddEndpoint(up.Endpoints[0])
	c.Assert(err, IsNil)

	_, err = s.backend.AddHost(host)
	c.Assert(err, IsNil)
	s.collectChanges(c, 4)

	loc := s.makeLocation("loc1", "/hello", host, up)

	_, err = s.backend.AddLocation(loc)
	c.Assert(err, IsNil)
	s.collectChanges(c, 1)

	options := LocationOptions{
		Limits: LocationLimits{
			MaxMemBodyBytes: 123456,
		},
	}

	locU, err := s.backend.UpdateLocationOptions(loc.Hostname, loc.Id, options)
	c.Assert(err, IsNil)
	c.Assert(locU.Options, DeepEquals, options)

	s.expectChanges(c, &LocationOptionsUpdated{
		Host:     host,
		Location: locU,
	})
}

func (s *EtcdBackendSuite) TestAddLocationBadUpstream(c *C) {
	host := s.makeHost("localhost")
	up1 := s.makeUpstream("u1", 1)
	loc := s.makeLocation("loc1", "/hello", host, up1)

	_, err := s.backend.AddLocation(loc)
	c.Assert(err, NotNil)
}

func (s *EtcdBackendSuite) TestAddLocationBadHost(c *C) {
	up := s.makeUpstream("u1", 1)
	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	host := s.makeHost("localhost")
	loc := s.makeLocation("loc1", "/hello", host, up)

	_, err = s.backend.AddLocation(loc)
	c.Assert(err, NotNil)
}

func (s *EtcdBackendSuite) TestLocationRateLimitCRUD(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)
	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)
	_, err = s.backend.AddHost(host)
	c.Assert(err, IsNil)
	s.collectChanges(c, 4)

	loc := s.makeLocation("loc1", "/hello", host, up)
	_, err = s.backend.AddLocation(loc)
	c.Assert(err, IsNil)
	s.collectChanges(c, 1)

	m := s.makeRateLimit("rl1", 10, "client.ip", 20, 1, loc)
	mR, err := s.backend.AddLocationMiddleware(loc.Hostname, loc.Id, m)
	c.Assert(mR, NotNil)
	c.Assert(err, IsNil)

	loc.Middlewares = []*MiddlewareInstance{m}
	s.expectChanges(c, &LocationMiddlewareUpdated{
		Host:       host,
		Location:   loc,
		Middleware: m,
	})

	_, err = s.backend.UpdateLocationMiddleware(loc.Hostname, loc.Id, m)
	c.Assert(err, IsNil)
	s.expectChanges(c, &LocationMiddlewareUpdated{
		Host:       host,
		Location:   loc,
		Middleware: m,
	})

	c.Assert(s.backend.DeleteLocationMiddleware(loc.Hostname, loc.Id, m.Type, m.Id), IsNil)
	loc.Middlewares = []*MiddlewareInstance{}
	s.expectChanges(c, &LocationMiddlewareDeleted{
		Host:           host,
		Location:       loc,
		MiddlewareId:   m.Id,
		MiddlewareType: m.Type,
	})
}

func (s *EtcdBackendSuite) TestLocationLimitsErrorHandling(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	loc := s.makeLocation("loc1", "/hello", host, up)

	// Location does not exist
	m := s.makeRateLimit("rl1", 10, "client.ip", 20, 1, loc)
	_, err := s.backend.AddLocationMiddleware(loc.Hostname, loc.Id, m)
	c.Assert(err, NotNil)

	_, err = s.backend.UpdateLocationMiddleware(loc.Hostname, loc.Id, m)
	c.Assert(err, NotNil)

	// Deeleteing non-existent middleware fails
	c.Assert(s.backend.DeleteLocationMiddleware(loc.Hostname, loc.Id, m.Type, m.Id), FitsTypeOf, &NotFoundError{})

	// Middleware type is not registered
	mBad := s.makeRateLimit("rl1", 10, "client.ip", 20, 1, loc)
	m.Type = "what"

	// Adding it fails
	_, err = s.backend.AddLocationMiddleware(loc.Hostname, loc.Id, mBad)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Updating it fails
	_, err = s.backend.UpdateLocationMiddleware(loc.Hostname, loc.Id, mBad)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Getting it fails
	_, err = s.backend.GetLocationMiddleware(loc.Hostname, loc.Id, mBad.Type, mBad.Id)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Deleting it fails
	c.Assert(s.backend.DeleteLocationMiddleware(loc.Hostname, loc.Id, "what", m.Id), FitsTypeOf, &NotFoundError{})

	// Just bad params
	_, err = s.backend.AddLocationMiddleware("", "", mBad)
	c.Assert(err, NotNil)

	// Updating it fails
	_, err = s.backend.UpdateLocationMiddleware("", "", mBad)
	c.Assert(err, NotNil)
}

func (s *EtcdBackendSuite) TestLocationMiddlewaresAutoId(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)
	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)
	_, err = s.backend.AddHost(host)
	c.Assert(err, IsNil)
	s.collectChanges(c, 3)

	loc := s.makeLocation("loc1", "/hello", host, up)
	_, err = s.backend.AddLocation(loc)
	c.Assert(err, IsNil)
	s.collectChanges(c, 1)

	m := s.makeRateLimit("", 10, "client.ip", 20, 1, loc)
	mR, err := s.backend.AddLocationMiddleware(loc.Hostname, loc.Id, m)
	c.Assert(err, IsNil)
	c.Assert(mR.Id, Not(Equals), "")
}

func (s *EtcdBackendSuite) TestDeleteUpstreamUsedByLocation(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]
	loc := s.makeLocation("loc1", "/hello", host, up)

	_, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)

	_, err = s.backend.AddHost(host)
	c.Assert(err, IsNil)

	_, err = s.backend.AddLocation(loc)
	c.Assert(err, IsNil)

	s.collectChanges(c, 4)
	c.Assert(s.backend.DeleteUpstream(up.Id), NotNil)
}

func (s *EtcdBackendSuite) makeUpstream(id string, endpoints int) *Upstream {
	up := &Upstream{
		Id:        id,
		Endpoints: []*Endpoint{},
	}

	for i := 1; i <= endpoints; i += 1 {
		e := &Endpoint{
			Id:         fmt.Sprintf("e%d", i),
			UpstreamId: up.Id,
			Url:        fmt.Sprintf("http://endpoint%d.com", i),
		}
		up.Endpoints = append(up.Endpoints, e)
	}
	return up
}

func (s *EtcdBackendSuite) makeHost(name string) *Host {
	return &Host{
		Name:      name,
		Locations: []*Location{},
		Listeners: []*Listener{},
		Options:   HostOptions{},
	}
}

func (s *EtcdBackendSuite) makeLocationWithOptions(id string, path string, host *Host, up *Upstream, options LocationOptions) *Location {
	return &Location{
		Id:          id,
		Hostname:    host.Name,
		Upstream:    up,
		Path:        path,
		Middlewares: []*MiddlewareInstance{},
		Options:     options,
	}
}

func (s *EtcdBackendSuite) makeLocation(id string, path string, host *Host, up *Upstream) *Location {
	return s.makeLocationWithOptions(id, path, host, up, LocationOptions{})
}

func (s *EtcdBackendSuite) makeRateLimit(id string, rate int64, variable string, burst int64, periodSeconds int64, loc *Location) *MiddlewareInstance {
	rl, err := ratelimit.FromOther(ratelimit.RateLimit{
		PeriodSeconds: periodSeconds,
		Requests: rate,
		Burst: burst,
		Variable: variable})
	if err != nil {
		panic(err)
	}
	return &MiddlewareInstance{
		Type:       "ratelimit",
		Priority:   1,
		Id:         id,
		Middleware: rl,
	}
}
