package registry

import (
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll/vulcan/middleware"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestLeaderRegistry(t *testing.T) {
	TestingT(t)
}

type LeaderSuite struct {
	client              *etcd.Client
	registry            *LeaderRegistry
	masterRegistration  *AppRegistration
	slaveRegistration   *AppRegistration
	handlerRegistration *HandlerRegistration
}

var _ = Suite(&LeaderSuite{})

func (s *LeaderSuite) SetUpSuite(c *C) {
	machines := []string{"http://127.0.0.1:4001"}
	s.client = etcd.NewClient(machines)
	s.registry = NewLeaderRegistry("customkey", "groupid", 15)
	s.masterRegistration = &AppRegistration{Name: "name", Host: "master", Port: 12345}
	s.slaveRegistration = &AppRegistration{Name: "name", Host: "slave", Port: 67890}
	s.handlerRegistration = &HandlerRegistration{
		Name:        "name",
		Host:        "host",
		Path:        "/path/to/server",
		Methods:     []string{"PUT"},
		Middlewares: []middleware.Middleware{middleware.Middleware{Type: "test", ID: "id", Spec: "hi"}},
	}
}

func (s *LeaderSuite) SetUpTest(c *C) {
	s.client.Delete("customkey", true)
}

func (s *LeaderSuite) TestRegisterAppCreatesBackend(c *C) {
	_ = s.registry.RegisterApp(s.masterRegistration)
	backend, err := s.client.Get("customkey/backends/name/backend", false, false)

	c.Assert(err, IsNil)
	c.Assert(backend.Node.Value, Equals, `{"Type":"http"}`)
	c.Assert(backend.Node.TTL, Equals, int64(0))
}

func (s *LeaderSuite) TestMasterServerRegistration(c *C) {
	_ = s.registry.RegisterApp(s.masterRegistration)

	server, err := s.client.Get("customkey/backends/name/servers/groupid", false, false)

	c.Assert(err, IsNil)
	c.Assert(server.Node.Value, Equals, `{"URL":"http://master:12345"}`)
	c.Assert(server.Node.TTL, Equals, int64(15))
}

func (s *LeaderSuite) TestSlaveServerRegistration(c *C) {
	master := NewLeaderRegistry("customkey", "groupid", 15)
	master.RegisterApp(s.masterRegistration)
	s.registry.RegisterApp(s.slaveRegistration)

	server, err := s.client.Get("customkey/backends/name/servers/groupid", false, false)

	c.Assert(err, IsNil)
	c.Assert(server.Node.Value, Equals, `{"URL":"http://master:12345"}`)
}

func (s *LeaderSuite) TestSlaveServerBecomesMaster(c *C) {
	// Create a master and slave.
	master := NewLeaderRegistry("customkey", "groupid", 15)
	master.RegisterApp(s.masterRegistration)
	s.registry.RegisterApp(s.slaveRegistration)

	// Remove the old master and re-register the slave.
	_, err := s.client.Delete("customkey/backends/name/servers/groupid", false)
	_ = s.registry.RegisterApp(s.slaveRegistration)
	_ = master.RegisterApp(s.masterRegistration)

	server, err := s.client.Get("customkey/backends/name/servers/groupid", false, false)

	c.Assert(err, IsNil)
	c.Assert(master.IsMaster, Equals, false)
	c.Assert(s.registry.IsMaster, Equals, true)
	c.Assert(server.Node.Value, Equals, `{"URL":"http://slave:67890"}`)
}

func (s *LeaderSuite) TestRegisterHandlerCreatesFrontend(c *C) {
	_ = s.registry.RegisterHandler(s.handlerRegistration)

	frontend, err := s.client.Get("customkey/frontends/host.put.path.to.server/frontend", false, false)

	c.Assert(err, IsNil)
	c.Assert(frontend.Node.Value, Matches, ".*path\\/to\\/server.*")
	c.Assert(frontend.Node.Value, Matches, ".*PUT.*")
	c.Assert(frontend.Node.TTL, Equals, int64(0))
}

func (s *LeaderSuite) TestRegisterHandlerCreatesMiddlewares(c *C) {
	_ = s.registry.RegisterHandler(s.handlerRegistration)

	frontend, err := s.client.Get("customkey/frontends/host.put.path.to.server/middlewares/id", false, false)

	c.Assert(err, IsNil)
	c.Assert(frontend.Node.Value, Matches, `{"Type":"test","Id":"id","Priority":0,"Middleware":"hi"}`)
	c.Assert(frontend.Node.TTL, Equals, int64(0))
}
