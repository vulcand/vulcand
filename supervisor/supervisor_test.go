package supervisor

import (
	"testing"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit/tokenbucket"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin/ratelimit"
	"github.com/mailgun/vulcand/server"
)

func TestConfigure(t *testing.T) { TestingT(t) }

type ConfSuite struct {
	conf *Configurator
}

func (s *ConfSuite) SetUpTest(c *C) {
	s.conf = NewConfigurator(&server.NopServer{})
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

func (s *ConfSuite) makeRateLimit(id string, rate int, variable string, burst int64, periodSeconds int, loc *Location) *MiddlewareInstance {
	rl, err := ratelimit.NewRateLimit(rate, variable, burst, periodSeconds)
	if err != nil {
		panic(err)
	}
	return &MiddlewareInstance{
		Type:       "ratelimit",
		Id:         id,
		Middleware: rl,
	}
}

func (s *ConfSuite) TestAddRemoveUpstreams(c *C) {
	location, _ := makeLocation()
	up := location.Upstream

	c.Assert(s.conf.processChange(&UpstreamAdded{up}), IsNil)
	c.Assert(s.conf.processChange(&UpstreamDeleted{UpstreamId: up.Id}), IsNil)
}

func (s *ConfSuite) TestUpdateRateLimit(c *C) {
	location, host := makeLocation()

	err := s.conf.processChange(&LocationAdded{Host: host, Location: location})
	c.Assert(err, IsNil)

	rl := s.makeRateLimit("rl1", 100, "client.ip", 200, 10, location)

	err = s.conf.processChange(&LocationMiddlewareAdded{Host: host, Location: location, Middleware: rl})
	c.Assert(err, IsNil)

	l := s.conf.getLocation(host.Name, location.Id)
	c.Assert(l, NotNil)

	// Make sure connection limit and rate limit are here as well
	chain := l.GetMiddlewareChain()
	limiter := chain.Get("ratelimit.rl1").(*tokenbucket.TokenLimiter)
	c.Assert(limiter.GetRate().Units, Equals, int64(100))
	c.Assert(limiter.GetRate().Period, Equals, time.Second*time.Duration(10))
	c.Assert(limiter.GetBurst(), Equals, int64(200))

	// Update the rate limit
	rl = s.makeRateLimit("rl1", 12, "client.ip", 20, 3, location)
	err = s.conf.processChange(&LocationMiddlewareUpdated{Host: host, Location: location, Middleware: rl})
	c.Assert(err, IsNil)

	// Make sure the changes have taken place
	limiter = chain.Get("ratelimit.rl1").(*tokenbucket.TokenLimiter)
	c.Assert(limiter.GetRate().Units, Equals, int64(12))
	c.Assert(limiter.GetRate().Period, Equals, time.Second*time.Duration(3))
	c.Assert(limiter.GetBurst(), Equals, int64(20))
}

func (s *ConfSuite) TestAddDeleteRateLimit(c *C) {
	location, host := makeLocation()

	err := s.conf.processChange(&LocationAdded{Host: host, Location: location})
	c.Assert(err, IsNil)

	rl := s.makeRateLimit("r1", 10, "client.ip", 1, 1, location)
	rl2 := s.makeRateLimit("r2", 10, "client.ip", 1, 1, location)

	err = s.conf.processChange(&LocationMiddlewareAdded{Host: host, Location: location, Middleware: rl})
	c.Assert(err, IsNil)

	err = s.conf.processChange(&LocationMiddlewareAdded{Host: host, Location: location, Middleware: rl2})
	c.Assert(err, IsNil)

	l := s.conf.getLocation(host.Name, location.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	chain := l.GetMiddlewareChain()
	c.Assert(chain.Get("ratelimit.r1"), NotNil)
	c.Assert(chain.Get("ratelimit.r2"), NotNil)

	err = s.conf.processChange(
		&LocationMiddlewareDeleted{
			Host:           host,
			Location:       location,
			MiddlewareId:   rl.Id,
			MiddlewareType: rl.Type,
		})
	c.Assert(err, IsNil)
	c.Assert(chain.Get("ratelimit.r1"), IsNil)
	// Make sure that the other rate limiter is still there
	c.Assert(chain.Get("ratelimit.r2"), NotNil)
}

func (s *ConfSuite) TestUpdateLocationPath(c *C) {
	location, host := makeLocation()

	err := s.conf.processChange(&LocationAdded{Host: host, Location: location})
	c.Assert(err, IsNil)

	// Host router matches inner router by hostname
	expRouter := s.conf.getRouter(host.Name)

	// Make sure that path router is configured correctly
	l := expRouter.GetLocationByExpression(convertPath(location.Path))
	c.Assert(l, NotNil)

	// Update location path
	oldPath := location.Path
	location.Path = "/new/path"
	err = s.conf.processChange(&LocationPathUpdated{Host: host, Location: location})
	c.Assert(err, IsNil)

	l = expRouter.GetLocationByExpression(convertPath(oldPath))
	c.Assert(l, IsNil)

	l = expRouter.GetLocationByExpression(convertPath(location.Path))
	c.Assert(l, NotNil)
}

// Make sure that update location path will actually create a location if it does not exist
func (s *ConfSuite) TestUpdateLocationPathUpsertsLocation(c *C) {
	location, host := makeLocation()

	err := s.conf.processChange(&LocationPathUpdated{Host: host, Location: location})
	c.Assert(err, IsNil)

	expRouter := s.conf.getRouter(host.Name)

	l := expRouter.GetLocationByExpression(convertPath(location.Path))
	c.Assert(l, NotNil)
}

func makeLocation() (*Location, *Host) {
	host := &Host{Name: "localhost"}
	upstream := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				Url: "http://localhost:5000",
			},
		},
	}
	location := &Location{
		Hostname: host.Name,
		Path:     "/home",
		Id:       "loc1",
		Upstream: upstream,
	}
	return location, host
}
