package server

import (
	"net/http"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit/tokenbucket"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/testutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/testutils"
	"testing"
)

func TestServer(t *testing.T) { TestingT(t) }

var _ = Suite(&ServerSuite{})

type ServerSuite struct {
	mux    *MuxServer
	lastId int
}

func (s *ServerSuite) SetUpTest(c *C) {
	m, err := NewMuxServerWithOptions(s.lastId, Options{})
	c.Assert(err, IsNil)
	s.mux = m
}

func (s *ServerSuite) TearDownTest(c *C) {
	s.mux.Stop(true)
}

func (s *ServerSuite) TestStartStop(c *C) {
	c.Assert(s.mux.Start(), IsNil)
}

func (s *ServerSuite) TestServerCRUD(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHost(h.Name), IsNil)

	_, _, err := GET(MakeURL(l, h.Listeners[0]), Opts{})
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestServerDefaultListener(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	defaultListener := &Listener{Protocol: HTTP, Address: Address{"tcp", "localhost:41000"}}

	m, err := NewMuxServerWithOptions(s.lastId, Options{DefaultListener: defaultListener})
	defer m.Stop(true)
	c.Assert(err, IsNil)
	s.mux = m

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})

	h.Listeners = []*Listener{}
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(s.mux.Start(), IsNil)
	c.Assert(GETResponse(c, MakeURL(l, defaultListener), Opts{}), Equals, "Hi, I'm endpoint")

}

// Test case when you have two hosts on the same socket
func (s *ServerSuite) TestTwoHosts(c *C) {
	e := NewTestResponder("Hi, I'm endpoint 1")
	defer e.Close()

	e2 := NewTestResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	l2, h2 := MakeLocation(LocOpts{Hostname: "otherhost", Addr: "localhost:31000", URL: e2.URL})
	c.Assert(s.mux.UpsertLocation(h2, l2), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint 1")
	c.Assert(GETResponse(c, MakeURL(l2, h2.Listeners[0]), Opts{Host: "otherhost"}), Equals, "Hi, I'm endpoint 2")
}

func (s *ServerSuite) TestServerListenerCRUD(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	h.Listeners = append(h.Listeners, &Listener{Id: "l2", Protocol: HTTP, Address: Address{"tcp", "localhost:31001"}})

	s.mux.AddHostListener(h, h.Listeners[1])

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[1]), Opts{}), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHostListener(h, h.Listeners[1].Id), IsNil)

	_, _, err := GET(MakeURL(l, h.Listeners[1]), Opts{})
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestDeleteHostListener(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	_, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: e.URL, URL: e.URL})
	c.Assert(s.mux.UpsertHost(h), NotNil)
	c.Assert(s.mux.DeleteHostListener(h, h.Listeners[0].Id), IsNil)
}

func (s *ServerSuite) TestServerHTTPSCRUD(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})
	h.KeyPair = &KeyPair{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHost(h.Name), IsNil)

	_, _, err := GET(MakeURL(l, h.Listeners[0]), Opts{})
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestLiveKeyPairUpdate(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()
	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})
	h.KeyPair = &KeyPair{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")

	h.KeyPair = &KeyPair{Key: localhostKey2, Cert: localhostCert2}
	c.Assert(s.mux.UpdateHostKeyPair(h.Name, h.KeyPair), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestSNI(c *C) {
	e := NewTestResponder("Hi, I'm endpoint 1")
	defer e.Close()

	e2 := NewTestResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)
	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})
	h.KeyPair = &KeyPair{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	l2, h2 := MakeLocation(LocOpts{Hostname: "otherhost", Addr: "localhost:31000", URL: e2.URL})
	h2.KeyPair = &KeyPair{Key: localhostKey2, Cert: localhostCert2}
	h2.Listeners[0].Protocol = HTTPS
	h2.Options.Default = true

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)
	c.Assert(s.mux.UpsertLocation(h2, l2), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint 1")
	c.Assert(GETResponse(c, MakeURL(l2, h.Listeners[0]), Opts{Host: "otherhost"}), Equals, "Hi, I'm endpoint 2")

	s.mux.DeleteHost(h2.Name)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint 1")

	response, _, err := GET(MakeURL(l, h2.Listeners[0]), Opts{Host: "otherhost"})
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Not(Equals), http.StatusOK)
}

func (s *ServerSuite) TestLocationProperties(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: "http://localhost:12345"})
	l.Middlewares = []*MiddlewareInstance{
		MakeRateLimit("rl1", 100, "client.ip", 200, 10, l),
	}
	l.Options = LocationOptions{
		Limits: LocationLimits{
			MaxBodyBytes: 123456,
		},
	}
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	// Make sure location is here
	loc := s.mux.getLocation(h.Name, l.Id)
	c.Assert(loc, NotNil)
	c.Assert(loc.GetOptions().Limits.MaxBodyBytes, Equals, int64(123456))

	// Make sure the endpoint has been added to the location
	lb := s.mux.locations[l.GetUniqueId()].getLB()
	c.Assert(lb, NotNil)

	// Check that endpoint is here
	endpoints := lb.GetEndpoints()
	c.Assert(len(endpoints), Equals, 1)
	AssertSameEndpoints(c, endpoints, l.Upstream.Endpoints)

	// Make sure connection limit and rate limit are here as well
	chain := loc.GetMiddlewareChain()
	c.Assert(chain.Get("ratelimit.rl1"), NotNil)

	// Delete the location
	c.Assert(s.mux.DeleteLocation(h, l.Id), IsNil)

	// Make sure it's no longer in the proxy
	loc = s.mux.getLocation(h.Name, l.Id)
	c.Assert(loc, IsNil)
}

func (s *ServerSuite) TestPerfMonitoring(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	e1 := NewTestResponder("Hi, I'm endpoint 1")
	defer e1.Close()

	e2 := NewTestResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e1.URL})
	l.Id = "loc1"

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)
	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint 1")

	// Make sure endpoint has been added to the performance monitor
	_, ok := s.mux.perfMon.locations[l.GetUniqueId().String()]
	c.Assert(ok, Equals, true)

	// Make sure upstream has been added to the performance monitor
	_, ok = s.mux.perfMon.upstreams[l.Upstream.GetUniqueId().String()]
	c.Assert(ok, Equals, true)

	// Delete the location
	c.Assert(s.mux.DeleteLocation(h, l.Id), IsNil)

	// Make sure location has been added to the performance monitor
	_, ok = s.mux.perfMon.locations[l.GetUniqueId().String()]
	c.Assert(ok, Equals, false)

	// Delete the upstream
	c.Assert(s.mux.DeleteUpstream(l.Upstream.Id), IsNil)

	_, ok = s.mux.perfMon.upstreams[l.Upstream.Id]
	c.Assert(ok, Equals, false)

	// Make sure all endpoints in the upstream have been deleted in the monitor
	_, ok = s.mux.perfMon.endpoints[l.Upstream.Endpoints[0].GetUniqueId().String()]
	c.Assert(ok, Equals, false)
}

func (s *ServerSuite) TestUpdateLocationOptions(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: "http://localhost:12345"})
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	l.Options = LocationOptions{
		Limits: LocationLimits{
			MaxBodyBytes: 123456,
		},
		FailoverPredicate: "IsNetworkError",
	}
	c.Assert(s.mux.UpdateLocationOptions(h, l), IsNil)

	lo := s.mux.getLocation(h.Name, l.Id)
	c.Assert(lo.GetOptions().FailoverPredicate, NotNil)
	c.Assert(lo.GetOptions().Limits.MaxBodyBytes, Equals, int64(123456))
}

func (s *ServerSuite) TestTrieRoutes(c *C) {
	e1 := NewTestResponder("Hi, I'm endpoint 1")
	defer e1.Close()

	e2 := NewTestResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l1, h1 := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e1.URL})
	l1.Path = `Path("/loc/path1")`
	l1.Id = "loc1"

	l2, h2 := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e2.URL})
	l2.Path = `Path("/loc/path2")`
	l2.Id = "loc2"

	c.Assert(s.mux.UpsertLocation(h1, l1), IsNil)
	c.Assert(s.mux.UpsertLocation(h2, l2), IsNil)

	c.Assert(GETResponse(c, "http://localhost:31000/loc/path1", Opts{}), Equals, "Hi, I'm endpoint 1")
	c.Assert(GETResponse(c, "http://localhost:31000/loc/path2", Opts{}), Equals, "Hi, I'm endpoint 2")
}

func (s *ServerSuite) TestUpdateLocationUpstream(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	e1 := NewTestResponder("1")
	defer e1.Close()

	e2 := NewTestResponder("2")
	defer e2.Close()

	e3 := NewTestResponder("3")
	defer e3.Close()

	h := &Host{
		Name:      "localhost",
		Listeners: []*Listener{&Listener{Protocol: HTTP, Address: Address{"tcp", "localhost:31000"}}},
	}
	up1 := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				Url: e1.URL,
			},
			{
				Url: e2.URL,
			},
		},
	}

	up2 := &Upstream{
		Id: "up2",
		Endpoints: []*Endpoint{
			{
				Url: e2.URL,
			},
			{
				Url: e3.URL,
			},
		},
	}

	l := &Location{
		Hostname: h.Name,
		Path:     "/loc1",
		Id:       "loc1",
		Upstream: up1,
	}

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	// Make sure the endpoint has been added to the location
	lb := s.mux.locations[l.GetUniqueId()].getLB()
	c.Assert(lb, NotNil)

	AssertSameEndpoints(c, lb.GetEndpoints(), up1.Endpoints)

	responseSet := make(map[string]bool)
	responseSet[GETResponse(c, "http://localhost:31000/loc1", Opts{})] = true
	responseSet[GETResponse(c, "http://localhost:31000/loc1", Opts{})] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true, "2": true})

	l.Upstream = up2

	c.Assert(s.mux.UpdateLocationUpstream(h, l), IsNil)

	AssertSameEndpoints(c, lb.GetEndpoints(), up2.Endpoints)

	responseSet = make(map[string]bool)
	responseSet[GETResponse(c, "http://localhost:31000/loc1", Opts{})] = true
	responseSet[GETResponse(c, "http://localhost:31000/loc1", Opts{})] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"2": true, "3": true})
}

func (s *ServerSuite) TestUpstreamEndpointCRUD(c *C) {
	e1 := NewTestResponder("1")
	defer e1.Close()

	e2 := NewTestResponder("2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e1.URL})

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	lb := s.mux.locations[l.GetUniqueId()].getLB()
	c.Assert(lb, NotNil)

	// Endpoints are taken from the upstream
	up := l.Upstream
	AssertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "1")

	// Add some endpoints to location
	newEndpoint := &Endpoint{
		Id:  e2.URL,
		Url: e2.URL,
	}
	up.Endpoints = append(up.Endpoints, newEndpoint)

	c.Assert(s.mux.UpsertEndpoint(up, newEndpoint), IsNil)

	// Endpoints have been updated in the load balancer
	AssertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints)

	// And actually work
	responseSet := make(map[string]bool)
	responseSet[GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{})] = true
	responseSet[GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{})] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true, "2": true})

	// Make sure endpoint has been added to the performance monitor after some requests
	// to it have been made
	_, ok := s.mux.perfMon.endpoints[newEndpoint.GetUniqueId().String()]
	c.Assert(ok, Equals, true)

	up.Endpoints = up.Endpoints[:1]
	c.Assert(s.mux.DeleteEndpoint(up, newEndpoint.Id), IsNil)

	// Make sure endpoint has been deleted from the performance monitor as well
	_, ok = s.mux.perfMon.endpoints[newEndpoint.GetUniqueId().String()]
	c.Assert(ok, Equals, false)

	AssertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints)

	// And actually work
	responseSet = make(map[string]bool)
	responseSet[GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{})] = true
	responseSet[GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{})] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true})
}

func (s *ServerSuite) TestUpstreamAddBadEndpoint(c *C) {
	e1 := NewTestResponder("1")
	defer e1.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e1.URL})

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	lb := s.mux.locations[l.GetUniqueId()].getLB()
	c.Assert(lb, NotNil)

	// Endpoints are taken from the upstream
	up := l.Upstream
	AssertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "1")

	// Add some endpoints to location
	newEndpoint := &Endpoint{
		Url: "http: local-host :500",
	}
	up.Endpoints = append(up.Endpoints, newEndpoint)

	c.Assert(s.mux.UpsertEndpoint(up, newEndpoint), NotNil)

	// Endpoints have not been updated in the load balancer
	AssertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints[:1])
}

func (s *ServerSuite) TestUpstreamUpdateEndpoint(c *C) {
	e1 := NewTestResponder("1")
	defer e1.Close()

	e2 := NewTestResponder("2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e1.URL})

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)
	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "1")

	ep := l.Upstream.Endpoints[0]
	ep.Url = e2.URL

	c.Assert(s.mux.UpsertEndpoint(l.Upstream, ep), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "2")
}

func (s *ServerSuite) TestUpdateRateLimit(c *C) {
	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: "http://localhost:32000"})
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	rl := MakeRateLimit("rl1", 100, "client.ip", 200, 10, l)

	c.Assert(s.mux.UpsertLocationMiddleware(h, l, rl), IsNil)

	loc := s.mux.getLocation(h.Name, l.Id)
	c.Assert(loc, NotNil)

	// Make sure connection limit and rate limit are here as well
	chain := loc.GetMiddlewareChain()
	limiter := chain.Get("ratelimit.rl1").(*tokenbucket.TokenLimiter)
	rs1 := tokenbucket.NewRateSet()
	rs1.Add(10*time.Second, 100, 200)
	c.Assert(limiter.DefaultRates(), DeepEquals, rs1)

	// Update the rate limit
	rl = MakeRateLimit("rl1", 12, "client.ip", 20, 3, l)
	c.Assert(s.mux.UpsertLocationMiddleware(h, l, rl), IsNil)

	// Make sure the changes have taken place
	limiter = chain.Get("ratelimit.rl1").(*tokenbucket.TokenLimiter)
	rs2 := tokenbucket.NewRateSet()
	rs2.Add(3*time.Second, 12, 20)
	c.Assert(limiter.DefaultRates(), DeepEquals, rs2)
}

func (s *ServerSuite) TestRateLimitCRUD(c *C) {
	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: "http://localhost:32000"})
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	rl := MakeRateLimit("r1", 10, "client.ip", 1, 1, l)
	rl2 := MakeRateLimit("r2", 10, "client.ip", 1, 1, l)

	c.Assert(s.mux.UpsertLocationMiddleware(h, l, rl), IsNil)
	c.Assert(s.mux.UpsertLocationMiddleware(h, l, rl2), IsNil)

	loc := s.mux.getLocation(h.Name, l.Id)
	c.Assert(loc, NotNil)

	chain := loc.GetMiddlewareChain()
	c.Assert(chain.Get("ratelimit.r1"), NotNil)
	c.Assert(chain.Get("ratelimit.r2"), NotNil)

	c.Assert(s.mux.DeleteLocationMiddleware(h, l, rl.Type, rl.Id), IsNil)

	c.Assert(chain.Get("ratelimit.r1"), IsNil)
	// Make sure that the other rate limiter is still there
	c.Assert(chain.Get("ratelimit.r2"), NotNil)
}

func (s *ServerSuite) TestUpdateLocationPath(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")

	l.Path = `Path("/hello/path2")`

	c.Assert(s.mux.UpdateLocationPath(h, l, l.Path), IsNil)

	c.Assert(GETResponse(c, "http://localhost:31000/hello/path2", Opts{}), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestUpdateLocationPathCreateLocation(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})

	c.Assert(s.mux.UpdateLocationPath(h, l, l.Path), IsNil)
	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestGetStats(c *C) {
	e1 := NewTestResponder("Hi, I'm endpoint 1")
	defer e1.Close()

	e2 := NewTestResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e1.URL})
	l.Upstream.Endpoints = []*Endpoint{
		{
			UpstreamId: l.Upstream.Id,
			Id:         e1.URL,
			Url:        e1.URL,
		},
		{
			UpstreamId: l.Upstream.Id,
			Id:         e2.URL,
			Url:        e2.URL,
		},
	}

	c.Assert(s.mux.UpdateLocationPath(h, l, l.Path), IsNil)
	for i := 0; i < 10; i++ {
		GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{})
	}

	stats, err := s.mux.GetEndpointStats(l.Upstream.Endpoints[0])
	c.Assert(err, IsNil)
	c.Assert(stats, NotNil)

	locStats, err := s.mux.GetLocationStats(l)
	c.Assert(locStats, NotNil)
	c.Assert(err, IsNil)

	upStats, err := s.mux.GetUpstreamStats(l.Upstream)
	c.Assert(upStats, NotNil)
	c.Assert(err, IsNil)

	topLocs, err := s.mux.GetTopLocations("", "")
	c.Assert(err, IsNil)
	c.Assert(len(topLocs), Equals, 1)

	topEndpoints, err := s.mux.GetTopEndpoints("")
	c.Assert(err, IsNil)
	c.Assert(len(topEndpoints), Equals, 2)
}

func (s *ServerSuite) TestUpdateUpstreamOptions(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("Hi, I'm backend"))
	})
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})
	l.Upstream.Options = UpstreamOptions{Timeouts: UpstreamTimeouts{Read: "1ms"}}
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	l1, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})
	l1.Upstream = l.Upstream
	c.Assert(s.mux.UpsertLocation(h, l1), IsNil)

	re, _, err := GET(MakeURL(l, h.Listeners[0]), Opts{})
	c.Assert(err, IsNil)
	c.Assert(re, NotNil)
	c.Assert(re.StatusCode, Equals, http.StatusRequestTimeout)

	re, _, err = GET(MakeURL(l1, h.Listeners[0]), Opts{})
	c.Assert(re.StatusCode, Equals, http.StatusRequestTimeout)

	l.Upstream.Options = UpstreamOptions{Timeouts: UpstreamTimeouts{Read: "20ms"}}
	c.Assert(s.mux.UpsertUpstream(l.Upstream), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm backend")
	c.Assert(GETResponse(c, MakeURL(l1, h.Listeners[0]), Opts{}), Equals, "Hi, I'm backend")
}

func (s *ServerSuite) TestSwitchUpstreams(c *C) {
	e := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("Hi, I'm backend"))
	})
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})
	l.Upstream.Options = UpstreamOptions{Timeouts: UpstreamTimeouts{Read: "10ms"}}
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	up := l.Upstream

	// This upstream is in use and can not be deleted
	c.Assert(s.mux.DeleteUpstream(up.Id), NotNil)

	re, _, err := GET(MakeURL(l, h.Listeners[0]), Opts{})
	c.Assert(err, IsNil)
	c.Assert(re, NotNil)
	c.Assert(re.StatusCode, Equals, http.StatusRequestTimeout)

	up2 := &Upstream{
		Id: "up2",
		Endpoints: []*Endpoint{
			{
				UpstreamId: "up2",
				Id:         e.URL,
				Url:        e.URL,
			},
		},
		Options: UpstreamOptions{
			Timeouts: UpstreamTimeouts{
				Read: "100ms",
			},
		},
	}

	l.Upstream = up2
	c.Assert(s.mux.UpdateLocationUpstream(h, l), IsNil)
	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm backend")

	// Upstream can now be deleted
	c.Assert(s.mux.DeleteUpstream(up.Id), IsNil)
}

func (s *ServerSuite) TestFilesNoFiles(c *C) {
	e := NewTestResponder("Hi, I'm endpoint 1")
	defer e.Close()

	files, err := s.mux.GetFiles()
	c.Assert(err, IsNil)
	c.Assert(len(files), Equals, 0)
	c.Assert(s.mux.Start(), IsNil)
}

func (s *ServerSuite) TestTakeFiles(c *C) {
	e := NewTestResponder("Hi, I'm endpoint 1")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e.URL})
	h.KeyPair = &KeyPair{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint 1")

	mux2, err := NewMuxServerWithOptions(s.lastId, Options{})
	c.Assert(err, IsNil)

	e2 := NewTestResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	l2, h2 := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:31000", URL: e2.URL})
	h2.KeyPair = &KeyPair{Key: localhostKey2, Cert: localhostCert2}
	h2.Listeners[0].Protocol = HTTPS

	c.Assert(mux2.UpsertLocation(h2, l2), IsNil)

	files, err := s.mux.GetFiles()
	c.Assert(err, IsNil)
	c.Assert(mux2.TakeFiles(files), IsNil)

	c.Assert(mux2.Start(), IsNil)
	s.mux.Stop(true)
	defer mux2.Stop(true)

	c.Assert(GETResponse(c, MakeURL(l2, h2.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint 2")
}

func AssertSameEndpoints(c *C, we []*roundrobin.WeightedEndpoint, e []*Endpoint) {
	if !EndpointsEq(we, e) {
		c.Fatalf("Expected endpoints sets to be the same %v and %v", we, e)
	}
}

func GETResponse(c *C, url string, opts Opts) string {
	response, body, err := GET(url, opts)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	return string(body)
}

// localhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at the last second of 2049 (the end
// of ASN.1 time).
// generated from src/pkg/crypto/tls:
// go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAN55NcYKZeInyTuhcCwFMhDHCmwaIUSdtXdcbItRB/yfXGBhiex0
0IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEAAQJBAQdUx66rfh8sYsgfdcvV
NoafYpnEcB5s4m/vSVe6SU7dCK6eYec9f9wpT353ljhDUHq3EbmE4foNzJngh35d
AekCIQDhRQG5Li0Wj8TM4obOnnXUXf1jRv0UkzE9AHWLG5q3AwIhAPzSjpYUDjVW
MCUXgckTpKCuGwbJk7424Nb8bLzf3kllAiA5mUBgjfr/WtFSJdWcPQ4Zt9KTMNKD
EUO0ukpTwEIl6wIhAMbGqZK3zAAFdq8DD2jPx+UJXnh0rnOkZBzDtJ6/iN69AiEA
1Aq8MJgTaYsDQWyU/hDq5YkDJc9e9DSCvUIzqxQWMQE=
-----END RSA PRIVATE KEY-----`)

var localhostCert2 = []byte(`-----BEGIN CERTIFICATE-----
MIIBizCCATegAwIBAgIRAL3EdJdBpGqcIy7kqCul6qIwCwYJKoZIhvcNAQELMBIx
EDAOBgNVBAoTB0FjbWUgQ28wIBcNNzAwMTAxMDAwMDAwWhgPMjA4NDAxMjkxNjAw
MDBaMBIxEDAOBgNVBAoTB0FjbWUgQ28wXDANBgkqhkiG9w0BAQEFAANLADBIAkEA
zAy3eIgjhro/wksSVgN+tZMxNbETDPgndYpIVSMMGHRXid71Zit8R5jJg8GZhWOs
2GXAZVZIJy634mODg5Xs8QIDAQABo2gwZjAOBgNVHQ8BAf8EBAMCAKQwEwYDVR0l
BAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zAuBgNVHREEJzAlggtleGFt
cGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATALBgkqhkiG9w0BAQsDQQA2
NW/PChPgBPt4q4ATTDDmoLoWjY8Vrp++6Wtue1YQBfEyvGWTFibNLD7FFodIPg/a
5LgeVKZTukSJX31lVCBm
-----END CERTIFICATE-----`)

var localhostKey2 = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBAMwMt3iII4a6P8JLElYDfrWTMTWxEwz4J3WKSFUjDBh0V4ne9WYr
fEeYyYPBmYVjrNhlwGVWSCcut+Jjg4OV7PECAwEAAQJAYHjOsZzj9wnNpUWrCKGk
YaKSzIjIsgQNW+QiKKZmTJS0rCJnUXUz8nSyTnS5rYd+CqOlFDXzpDbcouKGLOn5
BQIhAOtwl7+oebSLYHvznksQg66yvRxULfQTJS7aIKHNpDTPAiEA3d5gllV7EuGq
oqcbLwrFrGJ4WflasfeLpcDXuOR7sj8CIQC34IejuADVcMU6CVpnZc5yckYgCd6Z
8RnpLZKuy9yjIQIgYsykNk3agI39bnD7qfciD6HJ9kcUHCwgA6/cYHlenAECIQDZ
H4E4GFiDetx8ZOdWq4P7YRdIeepSvzPeOEv2sfsItg==
-----END RSA PRIVATE KEY-----`)
