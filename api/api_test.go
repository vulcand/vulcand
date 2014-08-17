package api

import (
	"net/http/httptest"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/gorilla/mux"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route/hostroute"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"github.com/mailgun/vulcand/adapter"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/backend/membackend"
	"github.com/mailgun/vulcand/configure"
	"github.com/mailgun/vulcand/plugin/connlimit"
	"github.com/mailgun/vulcand/plugin/registry"
)

func TestApi(t *testing.T) { TestingT(t) }

type ApiSuite struct {
	backend    Backend
	testServer *httptest.Server
	client     *Client
}

var _ = Suite(&ApiSuite{})

func (s *ApiSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})
}

func (s *ApiSuite) SetUpTest(c *C) {
	s.backend = membackend.NewMemBackend(registry.GetRegistry())

	muxRouter := mux.NewRouter()
	hostRouter := hostroute.NewHostRouter()
	proxy, err := vulcan.NewProxy(hostRouter)
	configurator := configure.NewConfigurator(proxy)
	c.Assert(err, IsNil)

	InitProxyController(s.backend, adapter.NewAdapter(proxy), configurator.GetConnWatcher(), muxRouter)
	s.testServer = httptest.NewServer(muxRouter)
	s.client = NewClient(s.testServer.URL, registry.GetRegistry())
}

func (s *ApiSuite) TearDownTest(c *C) {
	s.testServer.Close()
}

func (s *ApiSuite) TestStatus(c *C) {
	c.Assert(s.client.GetStatus(), IsNil)
}

func (s *ApiSuite) TestNotFoundHandler(c *C) {
	_, err := s.client.Get(s.client.endpoint("blabla"), nil)
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *ApiSuite) TestHostCRUD(c *C) {
	host, err := s.client.AddHost("localhost")
	c.Assert(err, IsNil)
	c.Assert(host.Name, Equals, "localhost")

	hosts, err := s.client.GetHosts()
	c.Assert(hosts, NotNil)
	c.Assert(err, IsNil)
	c.Assert(hosts[0].Name, Equals, "localhost")

	status, err := s.client.DeleteHost("localhost")
	c.Assert(err, IsNil)
	c.Assert(status, NotNil)

	hosts, err = s.client.GetHosts()
	c.Assert(len(hosts), Equals, 0)
	c.Assert(err, IsNil)
}

func (s *ApiSuite) TestAddHostTwice(c *C) {
	_, err := s.client.AddHost("localhost")
	c.Assert(err, IsNil)

	_, err = s.client.AddHost("localhost")
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
}

func (s *ApiSuite) TestDeleteHostNotFound(c *C) {
	_, err := s.client.DeleteHost("localhost")
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *ApiSuite) TestUpstreamCRUD(c *C) {
	up, err := s.client.AddUpstream("up1")
	c.Assert(err, IsNil)
	c.Assert(up.Id, Equals, "up1")

	out, err := s.client.GetUpstream("up1")
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out.Id, Equals, "up1")

	ups, err := s.client.GetUpstreams()
	c.Assert(err, IsNil)
	c.Assert(ups[0].Id, Equals, "up1")

	e, err := s.client.AddEndpoint("up1", "e1", "http://localhost:5000")
	c.Assert(err, IsNil)
	c.Assert(e.Id, Equals, "e1")

	_, err = s.client.DeleteEndpoint("up1", "e1")
	c.Assert(err, IsNil)

	_, err = s.client.DeleteUpstream("up1")
	c.Assert(err, IsNil)

	ups, err = s.client.GetUpstreams()
	c.Assert(err, IsNil)
	c.Assert(len(ups), Equals, 0)
}

func (s *ApiSuite) TestUpstreamHostTwice(c *C) {
	_, err := s.client.AddUpstream("up1")
	c.Assert(err, IsNil)

	_, err = s.client.AddUpstream("up1")
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
}

func (s *ApiSuite) TestDeleteUpstreamNotFound(c *C) {
	_, err := s.client.DeleteUpstream("where")
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *ApiSuite) TestGetUpstreamNotFound(c *C) {
	_, err := s.client.GetUpstream("where")
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *ApiSuite) TestUpstreamDrainConnections(c *C) {
	up, err := s.client.AddUpstream("up1")
	c.Assert(err, IsNil)

	_, err = s.client.AddEndpoint("up1", "e1", "http://localhost:5000")
	c.Assert(err, IsNil)

	conns, err := s.client.DrainUpstreamConnections(up.Id, "1")
	c.Assert(err, IsNil)
	c.Assert(conns, Equals, 0)
}

func (s *ApiSuite) TestLocationCRUD(c *C) {
	_, err := s.client.AddUpstream("up1")
	c.Assert(err, IsNil)

	_, err = s.client.AddEndpoint("up1", "e1", "http://localhost:5000")
	c.Assert(err, IsNil)

	_, err = s.client.AddHost("localhost")
	c.Assert(err, IsNil)

	loc, err := s.client.AddLocationWithOptions("localhost", "la", "/home", "up1", LocationOptions{Hostname: "somehost"})
	c.Assert(err, IsNil)
	c.Assert(loc, NotNil)
	c.Assert(loc.Hostname, Equals, "localhost")
	c.Assert(loc.Id, Equals, "la")
	c.Assert(loc.Path, Equals, "/home")
	c.Assert(loc.Upstream.Id, Equals, "up1")
	c.Assert(loc.Options.Hostname, Equals, "somehost")

	// Update location upstream
	_, err = s.client.AddUpstream("up2")
	c.Assert(err, IsNil)

	_, err = s.client.UpdateLocationUpstream("localhost", "la", "up2")
	c.Assert(err, IsNil)

	// Make sure changes have taken effect
	hosts, err := s.client.GetHosts()
	c.Assert(err, IsNil)
	c.Assert(hosts[0].Locations[0].Upstream.Id, Equals, "up2")

	// Delete a location
	_, err = s.client.DeleteLocation("localhost", "la")
	c.Assert(err, IsNil)

	// Check the result
	hosts, err = s.client.GetHosts()
	c.Assert(err, IsNil)
	c.Assert(len(hosts[0].Locations), Equals, 0)
}

func (s *ApiSuite) TestLocationUpdateOptions(c *C) {
	_, err := s.client.AddUpstream("up1")
	c.Assert(err, IsNil)

	_, err = s.client.AddEndpoint("up1", "e1", "http://localhost:5000")
	c.Assert(err, IsNil)

	_, err = s.client.AddHost("localhost")
	c.Assert(err, IsNil)

	loc, err := s.client.AddLocationWithOptions("localhost", "la", "/home", "up1", LocationOptions{Hostname: "somehost"})
	c.Assert(err, IsNil)
	c.Assert(loc, NotNil)

	// Update location upstream
	_, err = s.client.AddUpstream("up2")
	c.Assert(err, IsNil)

	_, err = s.client.UpdateLocationOptions("localhost", "la", LocationOptions{Hostname: "somehost2"})
	c.Assert(err, IsNil)

	// Make sure changes have taken effect
	hosts, err := s.client.GetHosts()
	c.Assert(err, IsNil)
	c.Assert(hosts[0].Locations[0].Options.Hostname, Equals, "somehost2")
}

func (s *ApiSuite) TestAddLocationTwice(c *C) {
	_, err := s.client.AddUpstream("up1")
	c.Assert(err, IsNil)

	_, err = s.client.AddEndpoint("up1", "e1", "http://localhost:5000")
	c.Assert(err, IsNil)

	_, err = s.client.AddHost("localhost")
	c.Assert(err, IsNil)

	_, err = s.client.AddLocation("localhost", "la", "/home", "up1")
	c.Assert(err, IsNil)

	_, err = s.client.AddLocation("localhost", "la", "/home", "up1")
	c.Assert(err, FitsTypeOf, &AlreadyExistsError{})
}

func (s *ApiSuite) TestAddLocationNoHost(c *C) {
	_, err := s.client.AddLocation("localhost", "la", "/home", "up1")
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *ApiSuite) TestAddLocationNoUpstream(c *C) {
	_, err := s.client.AddHost("localhost")
	c.Assert(err, IsNil)

	_, err = s.client.AddLocation("localhost", "la", "/home", "up1")
	c.Assert(err, FitsTypeOf, &NotFoundError{})
}

func (s *ApiSuite) TestMiddlewareCRUD(c *C) {
	_, err := s.client.AddUpstream("up1")
	c.Assert(err, IsNil)

	_, err = s.client.AddHost("localhost")
	c.Assert(err, IsNil)

	loc, err := s.client.AddLocation("localhost", "la", "/home", "up1")
	c.Assert(err, IsNil)
	c.Assert(loc, NotNil)

	cl := s.makeConnLimit("c1", 10, "client.ip", 2, loc)
	out, err := s.client.AddMiddleware(registry.GetRegistry().GetSpec(cl.Type), loc.Hostname, loc.Id, cl)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out.Id, Equals, cl.Id)
	c.Assert(out.Priority, Equals, cl.Priority)

	hosts, err := s.client.GetHosts()
	c.Assert(err, IsNil)
	m := hosts[0].Locations[0].Middlewares[0]
	c.Assert(m.Id, Equals, cl.Id)
	c.Assert(m.Type, Equals, cl.Type)
	c.Assert(m.Priority, Equals, cl.Priority)

	cl2 := s.makeConnLimit("c1", 10, "client.ip", 3, loc)
	out, err = s.client.UpdateMiddleware(registry.GetRegistry().GetSpec(cl.Type), loc.Hostname, loc.Id, cl2)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out.Id, Equals, cl2.Id)
	c.Assert(out.Priority, Equals, cl2.Priority)

	status, err := s.client.DeleteMiddleware(registry.GetRegistry().GetSpec(cl.Type), loc.Hostname, loc.Id, cl.Id)
	c.Assert(err, IsNil)
	c.Assert(status, NotNil)
}

func (s *ApiSuite) TestGetHosts(c *C) {
	hosts, err := s.client.GetHosts()
	c.Assert(err, IsNil)
	c.Assert(hosts, NotNil)
}

func (s *ApiSuite) makeConnLimit(id string, connections int64, variable string, priority int, loc *Location) *MiddlewareInstance {
	rl, err := connlimit.NewConnLimit(connections, variable)
	if err != nil {
		panic(err)
	}
	return &MiddlewareInstance{
		Type:       "connlimit",
		Id:         id,
		Middleware: rl,
	}
}
