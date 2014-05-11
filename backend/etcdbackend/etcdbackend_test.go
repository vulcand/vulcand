/*
NOTE:

github.com/davecgh/go-spew/spew package is extremely helpful

when it comes to debugging DeepEquals issues.

Here's how you'd use it:

spew.Printf("%#v\n vs\n %#v\n", a, b)
*/
package etcdbackend

import (
	"fmt"

	"github.com/mailgun/go-etcd/etcd"
	log "github.com/mailgun/gotools-log"
	. "github.com/mailgun/vulcand/backend"
	. "launchpad.net/gocheck"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConfigure(t *testing.T) { TestingT(t) }

type EtcdBackendSuite struct {
	backend     *EtcdBackend
	nodes       []string
	etcdPrefix  string
	consistency string
	client      *etcd.Client
	changesC    chan interface{}
}

var _ = Suite(&EtcdBackendSuite{etcdPrefix: "vulcandtest", consistency: etcd.STRONG_CONSISTENCY})

func (s *EtcdBackendSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	nodes_string := os.Getenv("VULCAND_ETCD_NODES")
	if nodes_string == "" {
		// Skips the entire suite
		c.Skip("This test requires etcd, provide comma separated nodes in VULCAND_TEST_ETCD_NODES environment variable")
		return
	}

	s.nodes = strings.Split(nodes_string, ",")
}

func (s *EtcdBackendSuite) SetUpTest(c *C) {
	// Initiate a backend
	backend, err := NewEtcdBackend(s.nodes, s.etcdPrefix, s.consistency)
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
	go s.backend.WatchChanges(s.changesC, false)
}

func (s *EtcdBackendSuite) TearDownTest(c *C) {
	// Make sure we've recognized the change
	s.backend.StopWatching()
}

func (s *EtcdBackendSuite) collectChanges(c *C, expected int) []interface{} {
	changes := make([]interface{}, expected)
	for i, _ := range changes {
		select {
		case changes[i] = <-s.changesC:
			//
		case <-time.After(time.Second):
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

	err := s.backend.AddHost(host.Name)
	c.Assert(err, IsNil)

	s.expectChanges(c, &HostAdded{Host: host})

	err = s.backend.DeleteHost("localhost")
	c.Assert(err, IsNil)

	s.expectChanges(c, &HostDeleted{
		Name:        "localhost",
		HostEtcdKey: host.EtcdKey,
	})
}

func (s *EtcdBackendSuite) TestAddBadHost(c *C) {
	// Add the host with empty hostname won't work
	err := s.backend.AddHost("")
	c.Assert(err, NotNil)
}

func (s *EtcdBackendSuite) TestGetters(c *C) {
	hosts, err := s.backend.GetHosts()
	c.Assert(err, IsNil)
	c.Assert(len(hosts), Equals, 0)

	upstreams, err := s.backend.GetUpstreams()
	c.Assert(err, IsNil)
	c.Assert(len(upstreams), Equals, 0)
}

func (s *EtcdBackendSuite) TestAddTwice(c *C) {
	// Add the host twice
	err := s.backend.AddHost("localhost")
	c.Assert(err, IsNil)

	err = s.backend.AddHost("localhost")
	c.Assert(err, NotNil)
}

func (s *EtcdBackendSuite) TestUpstreamCRUD(c *C) {
	err := s.backend.AddUpstream("up1")
	c.Assert(err, IsNil)

	s.expectChanges(c, &UpstreamAdded{
		Upstream: &Upstream{
			Id:        "up1",
			EtcdKey:   s.backend.path("upstreams", "up1"),
			Endpoints: []*Endpoint{}}})

	up, err := s.backend.GetUpstream("up1")
	c.Assert(err, IsNil)
	c.Assert(up, NotNil)
	c.Assert(up.Id, Equals, "up1")

	err = s.backend.DeleteUpstream("up1")
	c.Assert(err, IsNil)

	s.expectChanges(c, &UpstreamDeleted{
		UpstreamId:      "up1",
		UpstreamEtcdKey: s.backend.path("upstreams", "up1"),
	})
}

func (s *EtcdBackendSuite) TestUpstreamAutoId(c *C) {
	err := s.backend.AddUpstream("")
	c.Assert(err, IsNil)

	changes := s.collectChanges(c, 1)
	_, ok := changes[0].(*UpstreamAdded)
	c.Assert(ok, Equals, true)
}

func (s *EtcdBackendSuite) TestUpstreamTwice(c *C) {
	err := s.backend.AddUpstream("up1")
	c.Assert(err, IsNil)

	err = s.backend.AddUpstream("up1")
	c.Assert(err, NotNil)
}

func (s *EtcdBackendSuite) TestAddDeleteEndpoint(c *C) {
	up0 := s.makeUpstream("up1", 0)

	err := s.backend.AddUpstream(up0.Id)
	c.Assert(err, IsNil)

	s.expectChanges(c, &UpstreamAdded{Upstream: up0})
	up := s.makeUpstream("up1", 1)
	e := up.Endpoints[0]

	err = s.backend.AddEndpoint(up.Id, e.Id, e.Url)
	c.Assert(err, IsNil)

	s.expectChanges(c, &EndpointAdded{
		Upstream:          up,
		Endpoint:          e,
		AffectedLocations: []*Location{},
	})

	err = s.backend.DeleteEndpoint(up.Id, e.Id)
	c.Assert(err, IsNil)

	s.expectChanges(c, &EndpointDeleted{
		Upstream:          up0,
		EndpointId:        e.Id,
		EndpointEtcdKey:   e.EtcdKey,
		AffectedLocations: []*Location{},
	})
}

func (s *EtcdBackendSuite) TestAddEndpointUsingSet(c *C) {
	up := s.makeUpstream("u1", 1)
	e := up.Endpoints[0]

	_, err := s.client.Set(s.backend.path("upstreams", up.Id, "endpoints", e.Id), e.Url, 0)
	c.Assert(err, IsNil)

	s.expectChanges(c, &EndpointUpdated{
		Upstream:          up,
		Endpoint:          up.Endpoints[0],
		AffectedLocations: []*Location{},
	})
}

func (s *EtcdBackendSuite) TestAddEndpointAutoId(c *C) {
	up := s.makeUpstream("up1", 1)
	e := up.Endpoints[0]
	e.Id = ""

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)
}

func (s *EtcdBackendSuite) TestAddEndpointBadUrl(c *C) {
	up := s.makeUpstream("up1", 1)
	e := up.Endpoints[0]
	e.Id = ""

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, "http-definitely __== == not a good url"), NotNil)
}

func (s *EtcdBackendSuite) TestDeleteBadEndpoint(c *C) {
	up := s.makeUpstream("up1", 1)

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	// Non existent endpoint
	c.Assert(s.backend.DeleteEndpoint(up.Id, "notHere"), NotNil)
	// Non existent upstream
	c.Assert(s.backend.DeleteEndpoint("upNotHere", "notHere"), NotNil)
}

func (s *EtcdBackendSuite) TestAddDeleteLocation(c *C) {
	up := s.makeUpstream("u1", 1)
	e := up.Endpoints[0]
	c.Assert(s.backend.AddUpstream(up.Id), IsNil)

	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)

	host := s.makeHost("localhost")

	c.Assert(s.backend.AddHost(host.Name), IsNil)
	s.collectChanges(c, 3)

	loc := s.makeLocation("loc1", "/hello", host, up)

	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)

	s.expectChanges(c, &LocationUpstreamUpdated{
		Host:     host,
		Location: loc,
	})

	c.Assert(s.backend.DeleteLocation(loc.Hostname, loc.Id), IsNil)

	s.expectChanges(c, &LocationDeleted{
		Host:            host,
		LocationId:      loc.Id,
		LocationEtcdKey: loc.EtcdKey,
	})
}

// Make sure we can generate location id when it's not supplied
func (s *EtcdBackendSuite) TestLocationAutoId(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)
	c.Assert(s.backend.AddHost(host.Name), IsNil)
	s.collectChanges(c, 3)

	loc := s.makeLocation("", "/hello", host, up)

	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)
	s.collectChanges(c, 1)
}

// Update upstream of the location
func (s *EtcdBackendSuite) TestLocationUpdateUpstream(c *C) {
	up1 := s.makeUpstream("u1", 1)
	up2 := s.makeUpstream("u2", 1)

	host := s.makeHost("localhost")

	c.Assert(s.backend.AddUpstream(up1.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up1.Id, up1.Endpoints[0].Id, up1.Endpoints[0].Url), IsNil)

	c.Assert(s.backend.AddUpstream(up2.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up2.Id, up2.Endpoints[0].Id, up2.Endpoints[0].Url), IsNil)

	c.Assert(s.backend.AddHost(host.Name), IsNil)
	s.collectChanges(c, 5)

	loc := s.makeLocation("loc1", "/hello", host, up1)

	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)
	s.collectChanges(c, 1)

	c.Assert(s.backend.UpdateLocationUpstream(loc.Hostname, loc.Id, up2.Id), IsNil)

	loc.Upstream = up2
	s.expectChanges(c, &LocationUpstreamUpdated{
		Host:     host,
		Location: loc,
	})
}

func (s *EtcdBackendSuite) TestAddBadLocation(c *C) {
	// All empty params
	c.Assert(s.backend.AddLocation("", "", "", ""), NotNil)
	// Invalid path expression
	c.Assert(s.backend.AddLocation("loc1", "localhost", "   invalid path ~(**.*.\\", "up1"), NotNil)
	// Upstream does not exist
	c.Assert(s.backend.AddLocation("loc1", "localhost", "/home", "up1"), NotNil)
}

func (s *EtcdBackendSuite) TestLocationRateLimitCRUD(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)
	c.Assert(s.backend.AddHost(host.Name), IsNil)
	s.collectChanges(c, 3)

	loc := s.makeLocation("loc1", "/hello", host, up)

	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)
	s.collectChanges(c, 1)

	rl := s.makeRateLimit("rl1", 10, "client.ip", 20, 1, loc)
	c.Assert(s.backend.AddLocationRateLimit(loc.Hostname, loc.Id, rl.Id, rl), IsNil)

	loc.RateLimits = []*RateLimit{rl}
	s.expectChanges(c, &LocationRateLimitAdded{
		Host:      host,
		Location:  loc,
		RateLimit: rl,
	})

	rl.Burst = 100
	c.Assert(s.backend.UpdateLocationRateLimit(loc.Hostname, loc.Id, rl.Id, rl), IsNil)
	s.expectChanges(c, &LocationRateLimitUpdated{
		Host:      host,
		Location:  loc,
		RateLimit: rl,
	})

	c.Assert(s.backend.DeleteLocationRateLimit(loc.Hostname, loc.Id, rl.Id), IsNil)
	loc.RateLimits = []*RateLimit{}
	s.expectChanges(c, &LocationRateLimitDeleted{
		Host:             host,
		Location:         loc,
		RateLimitId:      rl.Id,
		RateLimitEtcdKey: rl.EtcdKey,
	})
}

func (s *EtcdBackendSuite) TestLocationLimitsBadLocation(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	loc := s.makeLocation("loc1", "/hello", host, up)

	// Location does not exist
	rl := s.makeRateLimit("rl1", 10, "client.ip", 20, 1, loc)
	c.Assert(s.backend.AddLocationRateLimit(host.Name, "loc2", rl.Id, rl), NotNil)

	cl := s.makeConnLimit("cl1", 10, "client.ip", loc)
	c.Assert(s.backend.AddLocationConnLimit(loc.Hostname, "loc2", cl.Id, cl), NotNil)

	// Updates will fail as well
	c.Assert(s.backend.UpdateLocationRateLimit(host.Name, "loc2", rl.Id, rl), NotNil)
	c.Assert(s.backend.UpdateLocationConnLimit(host.Name, "loc2", cl.Id, cl), NotNil)

	// Simply missing location
	c.Assert(s.backend.UpdateLocationRateLimit(host.Name, "", rl.Id, rl), NotNil)
	c.Assert(s.backend.UpdateLocationConnLimit(host.Name, "", cl.Id, cl), NotNil)

	// Limits do not exist, deleteing them should fail
	c.Assert(s.backend.DeleteLocationRateLimit(loc.Hostname, loc.Id, rl.Id), NotNil)
	c.Assert(s.backend.DeleteLocationConnLimit(loc.Hostname, loc.Id, cl.Id), NotNil)
}

func (s *EtcdBackendSuite) TestLocationRateLimitAutoId(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)
	c.Assert(s.backend.AddHost(host.Name), IsNil)
	s.collectChanges(c, 3)

	loc := s.makeLocation("loc1", "/hello", host, up)

	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)
	s.collectChanges(c, 1)

	rl := s.makeRateLimit("", 10, "client.ip", 20, 1, loc)
	c.Assert(s.backend.AddLocationRateLimit(loc.Hostname, loc.Id, rl.Id, rl), IsNil)

	loc.RateLimits = []*RateLimit{rl}
	changes := s.collectChanges(c, 1)
	change := changes[0].(*LocationRateLimitAdded)
	c.Assert(change.RateLimit.Id, Not(Equals), "")
}

func (s *EtcdBackendSuite) TestLocationConnLimitCRUD(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)
	c.Assert(s.backend.AddHost(host.Name), IsNil)
	s.collectChanges(c, 3)

	loc := s.makeLocation("loc1", "/hello", host, up)

	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)
	s.collectChanges(c, 1)

	cl := s.makeConnLimit("cl1", 10, "client.ip", loc)
	c.Assert(s.backend.AddLocationConnLimit(loc.Hostname, loc.Id, cl.Id, cl), IsNil)

	loc.ConnLimits = []*ConnLimit{cl}
	s.expectChanges(c, &LocationConnLimitAdded{
		Host:      host,
		Location:  loc,
		ConnLimit: cl,
	})

	cl.Connections = 100
	c.Assert(s.backend.UpdateLocationConnLimit(loc.Hostname, loc.Id, cl.Id, cl), IsNil)
	s.expectChanges(c, &LocationConnLimitUpdated{
		Host:      host,
		Location:  loc,
		ConnLimit: cl,
	})

	c.Assert(s.backend.DeleteLocationConnLimit(loc.Hostname, loc.Id, cl.Id), IsNil)

	loc.ConnLimits = []*ConnLimit{}
	s.expectChanges(c, &LocationConnLimitDeleted{
		Host:             host,
		Location:         loc,
		ConnLimitId:      cl.Id,
		ConnLimitEtcdKey: cl.EtcdKey,
	})
}

func (s *EtcdBackendSuite) TestLocationConnLimitAutoId(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)
	c.Assert(s.backend.AddHost(host.Name), IsNil)
	s.collectChanges(c, 3)

	loc := s.makeLocation("loc1", "/hello", host, up)

	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)
	s.collectChanges(c, 1)

	cl := s.makeConnLimit("", 10, "client.ip", loc)
	c.Assert(s.backend.AddLocationConnLimit(loc.Hostname, loc.Id, cl.Id, cl), IsNil)

	changes := s.collectChanges(c, 1)
	change := changes[0].(*LocationConnLimitAdded)
	c.Assert(change.ConnLimit.Id, Not(Equals), "")
}

func (s *EtcdBackendSuite) TestGenerateChanges(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]
	loc := s.makeLocation("loc1", "/hello", host, up)
	host.Locations = []*Location{loc}
	cl := s.makeConnLimit("cl1", 10, "client.ip", loc)
	rl := s.makeRateLimit("rl1", 10, "client.ip", 20, 1, loc)
	loc.RateLimits = []*RateLimit{rl}
	loc.ConnLimits = []*ConnLimit{cl}

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)
	c.Assert(s.backend.AddHost(host.Name), IsNil)
	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)
	c.Assert(s.backend.AddLocationConnLimit(loc.Hostname, loc.Id, cl.Id, cl), IsNil)
	c.Assert(s.backend.AddLocationRateLimit(loc.Hostname, loc.Id, rl.Id, rl), IsNil)

	backend, err := NewEtcdBackend(s.nodes, s.etcdPrefix, s.consistency)
	c.Assert(err, IsNil)
	defer backend.StopWatching()

	s.changesC = make(chan interface{})
	go s.backend.WatchChanges(s.changesC, true)
	s.expectChanges(c,
		&UpstreamAdded{Upstream: up},
		&EndpointAdded{Upstream: up, Endpoint: e},
		&HostAdded{Host: host},
		&LocationAdded{Host: host, Location: loc},
	)
}

func (s *EtcdBackendSuite) TestDeleteUpstreamUsedByLocation(c *C) {
	up := s.makeUpstream("u1", 1)
	host := s.makeHost("localhost")
	e := up.Endpoints[0]
	loc := s.makeLocation("loc1", "/hello", host, up)

	c.Assert(s.backend.AddUpstream(up.Id), IsNil)
	c.Assert(s.backend.AddEndpoint(up.Id, e.Id, e.Url), IsNil)
	c.Assert(s.backend.AddHost(host.Name), IsNil)
	c.Assert(s.backend.AddLocation(loc.Id, loc.Hostname, loc.Path, loc.Upstream.Id), IsNil)

	s.collectChanges(c, 4)

	c.Assert(s.backend.DeleteUpstream(up.Id), NotNil)
}

func (s *EtcdBackendSuite) makeUpstream(id string, endpoints int) *Upstream {
	up := &Upstream{
		Id:        id,
		EtcdKey:   s.backend.path("upstreams", id),
		Endpoints: []*Endpoint{},
	}

	for i := 1; i <= endpoints; i += 1 {
		e := &Endpoint{
			Id:      fmt.Sprintf("e%d", i),
			Url:     fmt.Sprintf("http://endpoint%d.com", i),
			EtcdKey: s.backend.path("upstreams", id, "endpoints", fmt.Sprintf("e%d", i)),
		}
		up.Endpoints = append(up.Endpoints, e)
	}
	return up
}

func (s *EtcdBackendSuite) makeHost(name string) *Host {
	return &Host{
		Name:      name,
		EtcdKey:   s.backend.path("hosts", name),
		Locations: []*Location{}}
}

func (s *EtcdBackendSuite) makeLocation(id string, path string, host *Host, up *Upstream) *Location {
	return &Location{
		Id:         id,
		EtcdKey:    s.backend.path("hosts", host.Name, "locations", id),
		Hostname:   host.Name,
		Upstream:   up,
		Path:       path,
		RateLimits: []*RateLimit{},
		ConnLimits: []*ConnLimit{},
	}
}

func (s *EtcdBackendSuite) makeRateLimit(id string, rate int, variable string, burst int, periodSeconds int, loc *Location) *RateLimit {
	rl, err := NewRateLimit(rate, variable, burst, periodSeconds)
	if err != nil {
		panic(err)
	}
	rl.Id = id
	rl.EtcdKey = s.backend.path("hosts", loc.Hostname, "locations", loc.Id, "limits", "rates", rl.Id)
	return rl
}

func (s *EtcdBackendSuite) makeConnLimit(id string, connections int, variable string, loc *Location) *ConnLimit {
	cl, err := NewConnLimit(connections, variable)
	if err != nil {
		panic(err)
	}
	cl.Id = id
	cl.EtcdKey = s.backend.path("hosts", loc.Hostname, "locations", loc.Id, "limits", "connections", cl.Id)
	return cl
}
