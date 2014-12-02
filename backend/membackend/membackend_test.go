package membackend

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin/ratelimit"
	. "github.com/mailgun/vulcand/plugin/registry"
	"testing"
)

func TestMemBackend(t *testing.T) { TestingT(t) }

type MemBackendSuite struct {
	backend Backend
}

var _ = Suite(&MemBackendSuite{})

func (s *MemBackendSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})
}

func (s *MemBackendSuite) SetUpTest(c *C) {
	s.backend = NewMemBackend(GetRegistry())
}

func (s *MemBackendSuite) TestGetRegistry(c *C) {
	c.Assert(s.backend.GetRegistry(), NotNil)
}

func (s *MemBackendSuite) TestGetHosts(c *C) {
	hosts, err := s.backend.GetHosts()
	c.Assert(hosts, NotNil)
	c.Assert(err, IsNil)
}

func (s *MemBackendSuite) TestHostOps(c *C) {
	h, err := NewHost("localhost")
	c.Assert(err, IsNil)

	out, err := s.backend.AddHost(h)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out.Name, Equals, h.Name)

	out2, err := s.backend.GetHost(h.Name)
	c.Assert(err, IsNil)
	c.Assert(out2.Name, Equals, h.Name)

	c.Assert(s.backend.DeleteHost(h.Name), IsNil)
	c.Assert(s.backend.DeleteHost(h.Name), FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestHostListenerOps(c *C) {
	h, err := NewHost("localhost")
	c.Assert(err, IsNil)

	_, err = s.backend.AddHost(h)
	c.Assert(err, IsNil)

	listener := &Listener{
		Protocol: "http",
		Address: Address{
			Network: "tcp",
			Address: "127.0.0.1:9000",
		},
	}
	_, err = s.backend.AddHostListener("localhost", listener)
	c.Assert(err, IsNil)

	_, err = s.backend.AddHostListener("localhost", listener)
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})

	c.Assert(s.backend.DeleteHostListener("localhost", listener.Id), IsNil)
	c.Assert(s.backend.DeleteHostListener("localhost", listener.Id), FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestAddHostTwice(c *C) {
	h, err := NewHost("localhost")
	c.Assert(err, IsNil)

	out, err := s.backend.AddHost(h)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)

	out2, err := s.backend.AddHost(h)
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
	c.Assert(out2, IsNil)
}

func (s *MemBackendSuite) TestUpstreamOps(c *C) {
	up, err := NewUpstream("up1")
	c.Assert(err, IsNil)
	c.Assert(up, NotNil)

	out, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out.Id, Equals, up.Id)

	ups, err := s.backend.GetUpstreams()
	c.Assert(err, IsNil)
	c.Assert(len(ups), Equals, 1)

	out2, err := s.backend.GetUpstream(up.Id)
	c.Assert(err, IsNil)
	c.Assert(out2, NotNil)
	c.Assert(out2.Id, Equals, up.Id)

	c.Assert(s.backend.DeleteUpstream(up.Id), IsNil)
	c.Assert(s.backend.DeleteUpstream(up.Id), FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestAddUpstreamAutoId(c *C) {
	up, err := NewUpstream("")
	c.Assert(err, IsNil)

	out, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)
	c.Assert(out.Id, Not(Equals), "")
}

func (s *MemBackendSuite) TestAddUpstreamTwice(c *C) {
	up, err := NewUpstream("up1")
	c.Assert(err, IsNil)
	c.Assert(up, NotNil)

	_, err = s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	_, err = s.backend.AddUpstream(up)
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
}

func (s *MemBackendSuite) TestEndpointOps(c *C) {
	up, err := NewUpstream("up1")
	c.Assert(err, IsNil)
	c.Assert(up, NotNil)

	out, err := s.backend.AddUpstream(up)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out.Id, Equals, up.Id)

	e, err := NewEndpoint(up.Id, "e1", "http://localhost:5000")
	c.Assert(err, IsNil)
	c.Assert(e, NotNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)

	e2, err := s.backend.GetEndpoint(up.Id, e.Id)
	c.Assert(err, IsNil)
	c.Assert(e2.Id, Equals, e.Id)
	c.Assert(e2.Url, Equals, e.Url)

	c.Assert(s.backend.DeleteEndpoint(up.Id, e.Id), IsNil)
	c.Assert(s.backend.DeleteEndpoint(up.Id, e.Id), FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestEndpointAutoId(c *C) {
	up, err := NewUpstream("up1")
	c.Assert(err, IsNil)

	_, err = s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	e, err := NewEndpoint(up.Id, "", "http://localhost:5000")
	c.Assert(err, IsNil)

	e2, err := s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)
	c.Assert(e2.Id, Not(Equals), "")
}

func (s *MemBackendSuite) TestEndpointAddEndpointTwice(c *C) {
	up, err := NewUpstream("up1")
	c.Assert(err, IsNil)

	_, err = s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	e, err := NewEndpoint(up.Id, "e1", "http://localhost:5000")
	c.Assert(err, IsNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
}

func (s *MemBackendSuite) TestEndpointAddNotFound(c *C) {
	e, err := NewEndpoint("up1", "e1", "http://localhost:5000")
	c.Assert(err, IsNil)
	c.Assert(e, NotNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestEndpointDeleteNotFound(c *C) {
	err := s.backend.DeleteEndpoint("up1", "e1")
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) setUp(c *C) (*Host, *Upstream) {
	h, err := NewHost("localhost")
	c.Assert(err, IsNil)

	up, err := NewUpstream("up1")
	c.Assert(err, IsNil)
	c.Assert(up, NotNil)

	e, err := NewEndpoint(up.Id, "e1", "http://localhost:5000")
	c.Assert(err, IsNil)
	c.Assert(e, NotNil)

	_, err = s.backend.AddUpstream(up)
	c.Assert(err, IsNil)

	_, err = s.backend.AddEndpoint(e)
	c.Assert(err, IsNil)

	_, err = s.backend.AddHost(h)
	c.Assert(err, IsNil)

	up.Endpoints = []*Endpoint{e}

	return h, up
}

func (s *MemBackendSuite) setUpLocation(c *C) (*Host, *Upstream, *Location) {
	h, up := s.setUp(c)

	l, err := NewLocation(h.Name, "loc1", "/home", up.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	_, err = s.backend.AddLocation(l)
	c.Assert(err, IsNil)
	return h, up, l
}

func (s *MemBackendSuite) TestLocationOps(c *C) {
	h, up := s.setUp(c)

	l, err := NewLocation(h.Name, "loc1", "/home", up.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	out, err := s.backend.AddLocation(l)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out.Id, Equals, l.Id)

	out2, err := s.backend.GetLocation(l.Hostname, l.Id)
	c.Assert(err, IsNil)
	c.Assert(out2, NotNil)
	c.Assert(out2.Id, Equals, l.Id)

	c.Assert(s.backend.DeleteLocation(l.Hostname, l.Id), IsNil)
	c.Assert(s.backend.DeleteLocation(l.Hostname, l.Id), FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestAddLocationTwice(c *C) {
	h, up := s.setUp(c)

	l, err := NewLocation(h.Name, "loc1", "/home", up.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	out, err := s.backend.AddLocation(l)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out.Id, Equals, l.Id)

	out, err = s.backend.AddLocation(l)
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
	c.Assert(out, IsNil)
}

func (s *MemBackendSuite) TestLocationNotFound(c *C) {
	h, _ := s.setUp(c)

	_, err := s.backend.GetLocation("", "")
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	_, err = s.backend.GetLocation(h.Name, "")
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestAddLocationBadHost(c *C) {
	_, up := s.setUp(c)

	l, err := NewLocation("oops", "loc1", "/home", up.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	_, err = s.backend.AddLocation(l)
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestAddLocationBadUpstream(c *C) {
	h, _ := s.setUp(c)

	l, err := NewLocation(h.Name, "loc1", "/home", "oops")
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	_, err = s.backend.AddLocation(l)
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestAddLocationAutoId(c *C) {
	h, up := s.setUp(c)

	l, err := NewLocation(h.Name, "", "/home", up.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	out, err := s.backend.AddLocation(l)
	c.Assert(err, IsNil)
	c.Assert(out.Id, Not(Equals), "")
}

func (s *MemBackendSuite) TestDeleteLocationUnknownHost(c *C) {
	c.Assert(s.backend.DeleteLocation("oops", "l1"), FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestLocationUpdateUpstream(c *C) {
	h, up := s.setUp(c)

	l, err := NewLocation(h.Name, "loc1", "/home", up.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	_, err = s.backend.AddLocation(l)
	c.Assert(err, IsNil)

	up2, err := NewUpstream("up2")
	c.Assert(err, IsNil)
	c.Assert(up2, NotNil)

	_, err = s.backend.AddUpstream(up2)
	c.Assert(err, IsNil)

	out, err := s.backend.UpdateLocationUpstream(h.Name, l.Id, up2.Id)
	c.Assert(err, IsNil)
	c.Assert(out.Upstream.Id, Equals, up2.Id)
}

func (s *MemBackendSuite) TestLocationUpdateUpstreamBadParams(c *C) {
	h, up := s.setUp(c)

	l, err := NewLocation(h.Name, "loc1", "/home", up.Id)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	_, err = s.backend.AddLocation(l)
	c.Assert(err, IsNil)

	// Host not found
	_, err = s.backend.UpdateLocationUpstream("oops", l.Id, up.Id)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Location not found
	_, err = s.backend.UpdateLocationUpstream(h.Name, "oops", up.Id)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Upstream not found
	_, err = s.backend.UpdateLocationUpstream(h.Name, l.Id, "oops")
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestLocationUpdateOptions(c *C) {
	h, _, loc := s.setUpLocation(c)

	o := LocationOptions{}
	l, err := s.backend.UpdateLocationOptions(h.Name, loc.Id, o)
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)
}

func (s *MemBackendSuite) TestLocationUpdateOptionsLocNotFound(c *C) {
	h, _, _ := s.setUpLocation(c)

	o := LocationOptions{}
	l, err := s.backend.UpdateLocationOptions(h.Name, "notfound", o)
	c.Assert(err, NotNil)
	c.Assert(l, IsNil)
}

func (s *MemBackendSuite) TestLocationMiddlewareOps(c *C) {
	h, _, loc := s.setUpLocation(c)

	rl := s.makeRateLimit("rl1", 10, "client.ip", 20, 1, loc)

	_, err := s.backend.AddLocationMiddleware(h.Name, loc.Id, rl)
	c.Assert(err, IsNil)

	out, err := s.backend.GetLocationMiddleware(h.Name, loc.Id, rl.Type, rl.Id)
	c.Assert(err, IsNil)
	c.Assert(out.Id, Equals, rl.Id)
	c.Assert(out.Type, Equals, rl.Type)

	rl2 := s.makeRateLimit("rl1", 20, "client.ip", 20, 1, loc)

	_, err = s.backend.UpdateLocationMiddleware(h.Name, loc.Id, rl2)
	c.Assert(err, IsNil)

	c.Assert(s.backend.DeleteLocationMiddleware(h.Name, loc.Id, rl2.Type, rl2.Id), IsNil)
	c.Assert(s.backend.DeleteLocationMiddleware(h.Name, loc.Id, rl2.Type, rl2.Id), FitsTypeOf, &NotFoundError{})

}

func (s *MemBackendSuite) TestGetLocationMiddlewareNotFound(c *C) {
	h, _, loc := s.setUpLocation(c)

	rl := s.makeRateLimit("rl1", 20, "client.ip", 20, 1, loc)

	// Middleware not found
	_, err := s.backend.GetLocationMiddleware(h.Name, loc.Id, rl.Type, rl.Id)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Location not found
	_, err = s.backend.GetLocationMiddleware(h.Name, "oops", rl.Type, rl.Id)
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestDeleteLocationMiddlewareNotFound(c *C) {
	h, _, loc := s.setUpLocation(c)

	rl := s.makeRateLimit("rl1", 20, "client.ip", 20, 1, loc)

	// Middleware not found
	err := s.backend.DeleteLocationMiddleware(h.Name, loc.Id, rl.Type, rl.Id)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Location not found
	err = s.backend.DeleteLocationMiddleware(h.Name, "oops", rl.Type, rl.Id)
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestUpdateLocationMiddlewareNotFound(c *C) {
	h, _, loc := s.setUpLocation(c)

	rl := s.makeRateLimit("rl1", 20, "client.ip", 20, 1, loc)

	// Middleware not found
	_, err := s.backend.UpdateLocationMiddleware(h.Name, loc.Id, rl)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Location not found
	_, err = s.backend.UpdateLocationMiddleware(h.Name, "oops", rl)
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) TestAddLocationMiddlewareAutoId(c *C) {
	h, _, loc := s.setUpLocation(c)

	rl := s.makeRateLimit("", 10, "client.ip", 20, 1, loc)

	out, err := s.backend.AddLocationMiddleware(h.Name, loc.Id, rl)
	c.Assert(err, IsNil)
	c.Assert(out.Id, Not(Equals), "")
}

func (s *MemBackendSuite) TestAddLocationMiddlewareTwice(c *C) {
	h, _, loc := s.setUpLocation(c)

	rl := s.makeRateLimit("rl", 10, "client.ip", 20, 1, loc)

	_, err := s.backend.AddLocationMiddleware(h.Name, loc.Id, rl)
	c.Assert(err, IsNil)

	_, err = s.backend.AddLocationMiddleware(h.Name, loc.Id, rl)
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
}

func (s *MemBackendSuite) TestAddLocationMiddlewareBadArgs(c *C) {
	h, _, loc := s.setUpLocation(c)

	rl := s.makeRateLimit("rl1", 10, "client.ip", 20, 1, loc)

	// Host not found
	_, err := s.backend.AddLocationMiddleware("oops", loc.Id, rl)
	c.Assert(err, FitsTypeOf, &NotFoundError{})

	// Location not found
	_, err = s.backend.AddLocationMiddleware(h.Name, "oops", rl)
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *MemBackendSuite) makeRateLimit(id string, rate int64, variable string, burst int64, periodSeconds int64, loc *Location) *MiddlewareInstance {
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
		Id:         id,
		Middleware: rl,
	}
}
