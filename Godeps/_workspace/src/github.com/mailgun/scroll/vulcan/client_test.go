package vulcan

import (
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll/vulcan/middleware"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestClient(t *testing.T) {
	TestingT(t)
}

type ClientSuite struct {
	etcd   *etcd.Client
	client *Client
}

var _ = Suite(&ClientSuite{})

func (s *ClientSuite) SetUpSuite(c *C) {
	machines := []string{"http://127.0.0.1:4001"}
	s.etcd = etcd.NewClient(machines)
	s.client = NewClient("clienttest")
}

func (s *ClientSuite) SetUpTest(c *C) {
	s.etcd.Delete("clienttest", true)
}

func (s *ClientSuite) TestCreateServer(c *C) {
	e, err := NewEndpointWithID("id", "name", "host", 8000)
	_ = s.client.CreateServer(e, 15)

	server, err := s.etcd.Get("clienttest/backends/name/servers/id", false, false)

	c.Assert(err, IsNil)
	c.Assert(server.Node.Value, Equals, `{"URL":"http://host:8000"}`)
	c.Assert(server.Node.TTL, Equals, int64(15))
}

func (s *ClientSuite) TestCreateDuplicateServer(c *C) {
	e, err := NewEndpointWithID("id", "name", "host", 8000)
	_ = s.client.CreateServer(e, 15)
	err = s.client.CreateServer(e, 15)

	c.Assert(err, ErrorMatches, ".*Key already exists.*")
}

func (s *ClientSuite) TestUpdateServer(c *C) {
	e, err := NewEndpointWithID("id", "name", "host", 8000)
	_ = s.client.CreateServer(e, 15)
	_ = s.client.UpdateServer(e, 15)

	server, err := s.etcd.Get("clienttest/backends/name/servers/id", false, false)

	c.Assert(err, IsNil)
	c.Assert(server.Node.Value, Equals, `{"URL":"http://host:8000"}`)
	c.Assert(server.Node.TTL, Equals, int64(15))
}

func (s *ClientSuite) TestUpdateDifferentServer(c *C) {
	e, err := NewEndpointWithID("id", "name", "host", 8000)
	_ = s.client.CreateServer(e, 15)

	e.URL = "differentURL"
	err = s.client.UpdateServer(e, 15)

	c.Assert(err, ErrorMatches, ".*Compare failed.*")
}

// Test Update Missing

func (s *ClientSuite) TestUpsertServer(c *C) {
	e, err := NewEndpointWithID("id", "name", "host", 8000)
	_ = s.client.UpsertServer(e, 15)

	server, err := s.etcd.Get("clienttest/backends/name/servers/id", false, false)

	c.Assert(err, IsNil)
	c.Assert(server.Node.Value, Equals, `{"URL":"http://host:8000"}`)
	c.Assert(server.Node.TTL, Equals, int64(15))
}

func (s *ClientSuite) TestRegisterBackend(c *C) {
	e, err := NewEndpointWithID("id", "name", "host", 8000)
	_ = s.client.RegisterBackend(e)

	backend, err := s.etcd.Get("clienttest/backends/name/backend", false, false)

	c.Assert(err, IsNil)
	c.Assert(backend.Node.Value, Equals, `{"Type":"http"}`)
	c.Assert(backend.Node.TTL, Equals, int64(0))
}

func (s *ClientSuite) TestRegisterFrontend(c *C) {
	m := []middleware.Middleware{middleware.Middleware{Type: "test", ID: "id", Spec: "hi"}}
	l := NewLocation("host", []string{"GET"}, "path/to/server", "name", m)
	_ = s.client.RegisterFrontend(l)

	frontend, err := s.etcd.Get("clienttest/frontends/host.getpath.to.server/frontend", false, false)

	c.Assert(err, IsNil)
	c.Assert(frontend.Node.Value, Matches, ".*path\\/to\\/server.*")
	c.Assert(frontend.Node.Value, Matches, ".*GET.*")
	c.Assert(frontend.Node.TTL, Equals, int64(0))
}

func (s *ClientSuite) TestRegisterHandlerCreatesMiddlewares(c *C) {
	m := []middleware.Middleware{middleware.Middleware{Type: "test", ID: "id", Spec: "hi"}}
	l := NewLocation("host", []string{"GET"}, "path/to/server", "name", m)
	_ = s.client.RegisterMiddleware(l)

	frontend, err := s.etcd.Get("clienttest/frontends/host.getpath.to.server/middlewares/id", false, false)

	c.Assert(err, IsNil)
	c.Assert(frontend.Node.Value, Matches, `{"Type":"test","Id":"id","Priority":0,"Middleware":"hi"}`)
	c.Assert(frontend.Node.TTL, Equals, int64(0))
}
