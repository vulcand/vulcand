package configure

import (
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/route/hostroute"
	. "github.com/mailgun/vulcand/backend"
	. "launchpad.net/gocheck"
	"testing"
)

func TestConfigure(t *testing.T) { TestingT(t) }

type ConfSuite struct {
	router *hostroute.HostRouter
	proxy  *vulcan.Proxy
	conf   *Configurator
}

func (s *ConfSuite) SetUpTest(c *C) {
	s.router = hostroute.NewHostRouter()
	proxy, err := vulcan.NewProxy(s.router)
	if err != nil {
		c.Fatal(err)
	}
	s.conf = NewConfigurator(proxy)
}

var _ = Suite(&ConfSuite{})

func (s *ConfSuite) AssertSameEndpoints(c *C, a []*roundrobin.WeightedEndpoint, b []*Endpoint) {
	x, y := map[string]bool{}, map[string]bool{}
	for _, e := range a {
		x[e.GetUrl().String()] = true
	}

	for _, e := range b {
		y[e.Url] = true
	}
	c.Assert(x, DeepEquals, y)
}

func (s *ConfSuite) TestAddDeleteHost(c *C) {
	host := &Host{Name: "localhost"}

	err := s.conf.processChange(&HostAdded{Host: host})
	c.Assert(err, IsNil)

	r, err := s.conf.a.GetPathRouter(host.Name)
	c.Assert(err, IsNil)
	c.Assert(r, NotNil)

	err = s.conf.processChange(&HostDeleted{Name: host.Name})
	c.Assert(err, IsNil)

	r, err = s.conf.a.FindPathRouter(host.Name)
	c.Assert(err, IsNil)
	c.Assert(r, IsNil)
}

func (s *ConfSuite) TestAddDeleteLocation(c *C) {
	host := &Host{Name: "localhost"}
	upstream := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				EtcdKey: "/up1/e1",
				Url:     "http://localhost:5000",
			},
		},
	}
	location := &Location{
		Hostname: host.Name,
		Path:     "/home",
		Id:       "loc1",
		Upstream: upstream,
		RateLimits: []*RateLimit{
			{
				Id:            "r1",
				PeriodSeconds: 1,
				Burst:         1,
				Variable:      "client.ip",
				Requests:      10,
				EtcdKey:       "/r1",
			},
		},
		ConnLimits: []*ConnLimit{
			{
				Id:          "c1",
				Variable:    "client.ip",
				Connections: 10,
				EtcdKey:     "/c1",
			},
		},
	}

	err := s.conf.processChange(&LocationAdded{Host: host, Location: location})
	c.Assert(err, IsNil)

	// Make sure location is here
	l, err := s.conf.a.GetHttpLocation(host.Name, location.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	// Make sure the endpoint has been added to the location
	lb, err := s.conf.a.GetHttpLocationLb(host.Name, location.Id)
	c.Assert(err, IsNil)
	c.Assert(lb, NotNil)

	// Check that endpoint is here
	endpoints := lb.GetEndpoints()
	c.Assert(len(endpoints), Equals, 1)
	s.AssertSameEndpoints(c, endpoints, upstream.Endpoints)

	// Make sure connection limit and rate limit are here as well
	chain := l.GetMiddlewareChain()
	c.Assert(chain.Get("/c1"), NotNil)
	c.Assert(chain.Get("/r1"), NotNil)

	// Delete the location
	err = s.conf.processChange(&LocationDeleted{Host: host, LocationId: location.Id})
	c.Assert(err, IsNil)

	// Make sure it's no longer in the proxy
	l, err = s.conf.a.FindHttpLocation(host.Name, location.Id)
	c.Assert(err, IsNil)
	c.Assert(l, IsNil)
}

func (s *ConfSuite) TestUpdateLocationUpstream(c *C) {
	host := &Host{Name: "localhost"}
	up1 := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				EtcdKey: "/up1/e1",
				Url:     "http://localhost:5000",
			},
			{
				EtcdKey: "/up1/e2",
				Url:     "http://localhost:5001",
			},
		},
	}

	up2 := &Upstream{
		Id: "up2",
		Endpoints: []*Endpoint{
			{
				EtcdKey: "/up2/e1",
				Url:     "http://localhost:5001",
			},
			{
				Id:  "/up2/e2",
				Url: "http://localhost:5002",
			},
		},
	}

	location := &Location{
		Hostname: host.Name,
		Path:     "/home",
		Id:       "loc1",
		Upstream: up1,
	}

	err := s.conf.processChange(&LocationAdded{Host: host, Location: location})
	c.Assert(err, IsNil)

	// Make sure the endpoint has been added to the location
	lb, err := s.conf.a.GetHttpLocationLb(host.Name, location.Id)
	c.Assert(err, IsNil)
	c.Assert(lb, NotNil)

	// Endpoints are taken from up1
	s.AssertSameEndpoints(c, lb.GetEndpoints(), up1.Endpoints)

	location.Upstream = up2
	err = s.conf.processChange(
		&LocationUpstreamUpdated{Host: host, Location: location, UpstreamId: "u2"})
	c.Assert(err, IsNil)

	// Endpoints are taken from up2
	s.AssertSameEndpoints(c, lb.GetEndpoints(), up2.Endpoints)
}
