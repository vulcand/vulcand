package configure

import (
	"github.com/mailgun/vulcan"
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
				Id:  "e1",
				Url: "http://localhost:5000",
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
				Id:            "loc1",
				PeriodSeconds: 1,
				Burst:         1,
				Variable:      "client.ip",
				Requests:      10,
			},
		},
	}

	err := s.conf.processChange(&LocationAdded{Host: host, Location: location})
	c.Assert(err, IsNil)

	l, err := s.conf.a.GetHttpLocation(host.Name, location.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)
}
