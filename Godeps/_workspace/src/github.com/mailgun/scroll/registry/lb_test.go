package registry

import (
	"fmt"
	"os"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll/vulcan/middleware"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestLBRegistry(t *testing.T) {
	TestingT(t)
}

type LBRegistrySuite struct {
	client              *etcd.Client
	registry            *LBRegistry
	appRegistration     *AppRegistration
	handlerRegistration *HandlerRegistration
}

var _ = Suite(&LBRegistrySuite{})

func (s *LBRegistrySuite) SetUpSuite(c *C) {
	machines := []string{"http://127.0.0.1:4001"}
	s.client = etcd.NewClient(machines)
	s.client.Delete("customkey", true)

	s.registry, _ = NewLBRegistry("customkey", 15)
	s.appRegistration = &AppRegistration{Name: "name", Host: "host", Port: 12345}
	s.handlerRegistration = &HandlerRegistration{
		Name:        "name",
		Host:        "host",
		Path:        "/path/to/server",
		Methods:     []string{"PUT"},
		Middlewares: []middleware.Middleware{middleware.Middleware{Type: "test", ID: "id", Spec: "hi"}},
	}
}

func (s *LBRegistrySuite) TestRegisterAppCreatesBackend(c *C) {
	_ = s.registry.RegisterApp(s.appRegistration)
	backend, err := s.client.Get("customkey/backends/name/backend", false, false)

	c.Assert(err, IsNil)
	c.Assert(backend.Node.Value, Equals, `{"Type":"http"}`)
	c.Assert(backend.Node.TTL, Equals, int64(0))
}

func (s *LBRegistrySuite) TestRegisterAppCreatesServer(c *C) {
	_ = s.registry.RegisterApp(s.appRegistration)

	host, err := os.Hostname()
	key := fmt.Sprintf("customkey/backends/name/servers/%s_12345", host)
	server, err := s.client.Get(key, false, false)

	c.Assert(err, IsNil)
	c.Assert(server.Node.Value, Equals, `{"URL":"http://host:12345"}`)
	c.Assert(server.Node.TTL, Equals, int64(15))
}

func (s *LBRegistrySuite) TestRegisterHandlerCreatesFrontend(c *C) {
	_ = s.registry.RegisterHandler(s.handlerRegistration)

	frontend, err := s.client.Get("customkey/frontends/host.put.path.to.server/frontend", false, false)

	c.Assert(err, IsNil)
	c.Assert(frontend.Node.Value, Matches, ".*path\\/to\\/server.*")
	c.Assert(frontend.Node.Value, Matches, ".*PUT.*")
	c.Assert(frontend.Node.TTL, Equals, int64(0))
}

func (s *LBRegistrySuite) TestRegisterHandlerCreatesMiddlewares(c *C) {
	_ = s.registry.RegisterHandler(s.handlerRegistration)

	frontend, err := s.client.Get("customkey/frontends/host.put.path.to.server/middlewares/id", false, false)

	c.Assert(err, IsNil)
	c.Assert(frontend.Node.Value, Matches, `{"Type":"test","Id":"id","Priority":0,"Middleware":"hi"}`)
	c.Assert(frontend.Node.TTL, Equals, int64(0))
}
