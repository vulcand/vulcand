package backend

import (
	"encoding/json"
	"github.com/mailgun/vulcand/plugin/connlimit"
	"github.com/mailgun/vulcand/plugin/registry"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/launchpad.net/gocheck"
	"testing"
)

func TestBackend(t *testing.T) { TestingT(t) }

type BackendSuite struct {
}

var _ = Suite(&BackendSuite{})

func (s *BackendSuite) TestNewHost(c *C) {
	h, err := NewHost("localhost")
	c.Assert(err, IsNil)
	c.Assert(h.Name, Equals, "localhost")
	c.Assert(h.Name, Equals, h.GetId())
	c.Assert(h.String(), Not(Equals), "")
}

func (s *BackendSuite) TestNewHostBad(c *C) {
	h, err := NewHost("")
	c.Assert(err, NotNil)
	c.Assert(h, IsNil)
}

func (s *BackendSuite) TestNewLocation(c *C) {
	l, err := NewLocation("localhost", "loc1", "/home", "u1")
	c.Assert(err, IsNil)
	c.Assert(l.GetId(), Equals, "loc1")
	c.Assert(l.String(), Not(Equals), "")
}

func (s *BackendSuite) TestNewLocationBadParams(c *C) {
	// Bad path
	_, err := NewLocation("localhost", "loc1", "** /home  -- afawf \\~", "u1")
	c.Assert(err, NotNil)

	// Empty params
	_, err = NewLocation("", "", "", "")
	c.Assert(err, NotNil)
}

func (s *BackendSuite) TestNewUpstream(c *C) {
	u, err := NewUpstream("u1")
	c.Assert(err, IsNil)
	c.Assert(u.GetId(), Equals, "u1")
	c.Assert(u.String(), Not(Equals), "")
}

func (s *BackendSuite) TestNewEndpoint(c *C) {
	e, err := NewEndpoint("u1", "e1", "http://localhost")
	c.Assert(err, IsNil)
	c.Assert(e.GetId(), Equals, "e1")
	c.Assert(e.String(), Not(Equals), "")
}

func (s *BackendSuite) TestNewEndpointBadParams(c *C) {
	_, err := NewEndpoint("u1", "e1", "http---")
	c.Assert(err, NotNil)

	// Missing upstream
	_, err = NewEndpoint("", "e1", "http://localhost")
	c.Assert(err, NotNil)
}

func (s *BackendSuite) TestHostsFromJson(c *C) {
	h, err := NewHost("localhost")
	c.Assert(err, IsNil)

	up, err := NewUpstream("up1")
	c.Assert(err, IsNil)

	e, err := NewEndpoint("u1", "e1", "http://localhost")
	c.Assert(err, IsNil)

	l, err := NewLocation("localhost", "loc1", "/path", "up1")
	c.Assert(err, IsNil)

	cl, err := connlimit.NewConnLimit(10, "client.ip")
	c.Assert(err, IsNil)

	i := &MiddlewareInstance{Id: "c1", Type: "connlimit", Middleware: cl}

	up.Endpoints = []*Endpoint{e}
	l.Upstream = up
	l.Middlewares = []*MiddlewareInstance{i}
	h.Locations = []*Location{l}
	hosts := []*Host{h}

	bytes, err := json.Marshal(map[string]interface{}{"Hosts": hosts})

	out, err := HostsFromJson(bytes, registry.GetRegistry().GetSpec)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)

	c.Assert(out, DeepEquals, hosts)
}

func (s *BackendSuite) TestUpstreamFromJson(c *C) {
	up, err := NewUpstream("up1")
	c.Assert(err, IsNil)

	e, err := NewEndpoint("u1", "e1", "http://localhost")
	c.Assert(err, IsNil)

	up.Endpoints = []*Endpoint{e}

	bytes, err := json.Marshal(up)
	c.Assert(err, IsNil)

	out, err := UpstreamFromJson(bytes)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)

	c.Assert(out, DeepEquals, up)
}

func (s *BackendSuite) TestEndpointFromJson(c *C) {
	e, err := NewEndpoint("u1", "e1", "http://localhost")
	c.Assert(err, IsNil)

	bytes, err := json.Marshal(e)
	c.Assert(err, IsNil)

	out, err := EndpointFromJson(bytes)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)

	c.Assert(out, DeepEquals, e)
}
