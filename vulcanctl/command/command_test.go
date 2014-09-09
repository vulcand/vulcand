package command

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/gorilla/mux"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"github.com/mailgun/vulcand/api"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/backend/membackend"
	"github.com/mailgun/vulcand/connwatch"
	"github.com/mailgun/vulcand/plugin/registry"
	"github.com/mailgun/vulcand/server"
	"github.com/mailgun/vulcand/supervisor"
)

const OK = ".*OK.*"

func TestVulcanCommandLineTool(t *testing.T) { TestingT(t) }

type CmdSuite struct {
	backend    Backend
	out        *bytes.Buffer
	cmd        *Command
	testServer *httptest.Server
}

var _ = Suite(&CmdSuite{})

func (s *CmdSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})
}

func (s *CmdSuite) SetUpTest(c *C) {
	s.backend = membackend.NewMemBackend(registry.GetRegistry())

	newServer := func(id int, cw *connwatch.ConnectionWatcher) (server.Server, error) {
		return server.NewMuxServerWithOptions(id, cw, server.Options{})
	}

	newBackend := func() (Backend, error) {
		return s.backend, nil
	}

	sv := supervisor.NewSupervisor(newServer, newBackend, make(chan error))
	muxRouter := mux.NewRouter()

	api.InitProxyController(s.backend, sv, sv.GetConnWatcher(), muxRouter)
	s.testServer = httptest.NewServer(muxRouter)

	s.out = &bytes.Buffer{}
	s.cmd = &Command{registry: registry.GetRegistry(), out: s.out, vulcanUrl: s.testServer.URL}
}

func (s *CmdSuite) runString(in string) string {
	return s.run(strings.Split(in, " ")...)
}

func (s *CmdSuite) run(params ...string) string {
	args := []string{"vulcanctl"}
	args = append(args, params...)
	args = append(args, fmt.Sprintf("--vulcan=%s", s.testServer.URL))
	s.out = &bytes.Buffer{}
	s.cmd = &Command{registry: registry.GetRegistry(), out: s.out, vulcanUrl: s.testServer.URL}
	s.cmd.Run(args)
	return strings.Replace(s.out.String(), "\n", " ", -1)
}

func (s *CmdSuite) TestStatus(c *C) {
	c.Assert(s.run("status"), Matches, ".*hosts.*")
}

func (s *CmdSuite) TestHostCRUD(c *C) {
	host := "host"
	c.Assert(s.run("host", "add", "-name", host), Matches, OK)
	c.Assert(s.run("host", "rm", "-name", host), Matches, OK)
}

func (s *CmdSuite) TestHostListenerCRUD(c *C) {
	host := "host"
	c.Assert(s.run("host", "add", "-name", host), Matches, OK)
	l := "l1"
	c.Assert(s.run("listener", "add", "-host", host, "-id", l, "-proto", "http", "-addr", "localhost:11300"), Matches, OK)
	c.Assert(s.run("listener", "rm", "-host", host, "-id", l), Matches, OK)
	c.Assert(s.run("host", "rm", "-name", host), Matches, OK)
}

func (s *CmdSuite) TestUpstreamCRUD(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, OK)
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, OK)
	c.Assert(s.run("upstream", "ls"), Matches, fmt.Sprintf(".*%s.*", up))
}

func (s *CmdSuite) TestUpstreamAutoId(c *C) {
	c.Assert(s.run("upstream", "add"), Matches, OK)
}

func (s *CmdSuite) TestEndpointCRUD(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, OK)
	e := "e"
	c.Assert(s.run("endpoint", "add", "-id", e, "-url", "http://localhost:5000", "-up", up), Matches, OK)

	c.Assert(s.run("endpoint", "rm", "-id", e, "-up", up), Matches, OK)
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, OK)
}

func (s *CmdSuite) TestLimitsCRUD(c *C) {
	// Create upstream with this location
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, OK)

	h := "h"
	c.Assert(s.run("host", "add", "-name", h), Matches, OK)

	loc := "loc"
	path := "/path"
	c.Assert(s.run("location", "add", "-host", h, "-id", loc, "-up", up, "-path", path), Matches, OK)

	rl := "rl"
	c.Assert(s.run("ratelimit", "add", "-host", h, "-loc", loc, "-id", rl, "-requests", "10", "-variable", "client.ip", "-period", "3"), Matches, OK)
	c.Assert(s.run("ratelimit", "update", "-host", h, "-loc", loc, "-id", rl, "-requests", "100", "-variable", "client.ip", "-period", "30"), Matches, OK)
	c.Assert(s.run("ratelimit", "rm", "-host", h, "-loc", loc, "-id", rl), Matches, OK)

	cl := "cl"
	c.Assert(s.run("connlimit", "add", "-host", h, "-loc", loc, "-id", cl, "-connections", "10", "-variable", "client.ip"), Matches, OK)
	c.Assert(s.run("connlimit", "update", "-host", h, "-loc", loc, "-id", cl, "-connections", "100", "-variable", "client.ip"), Matches, OK)
	c.Assert(s.run("connlimit", "rm", "-host", h, "-loc", loc, "-id", cl), Matches, OK)

	c.Assert(s.run("location", "rm", "-host", h, "-id", loc), Matches, OK)
	c.Assert(s.run("host", "rm", "-name", h), Matches, OK)
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, OK)
}

func (s *CmdSuite) TestUpstreamDrainConnections(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, OK)
	c.Assert(s.run("upstream", "drain", "--id", up, "--timeout", "0"), Matches, ".*Connections: 0.*")
}

func (s *CmdSuite) TestLocationOptions(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, OK)

	h := "h"
	c.Assert(s.run("host", "add", "-name", h), Matches, OK)

	loc := "loc"
	path := "/path"
	c.Assert(s.run(
		"location", "add",
		"-host", h, "-id", loc, "-up", up, "-path", path,
		// Timeouts
		"-readTimeout", "1s", "-dialTimeout", "2s", "-handshakeTimeout", "3s",
		// Keep Alive parameters
		"-keepAlivePeriod", "4s", "-maxIdleConns", "5",
		// Limits
		"-maxMemBodyKB", "6", "-maxBodyKB", "7",
		// Misc parameters
		// Failover predicate
		"-failoverPredicate", "IsNetworkError",
		// Forward header
		"-trustForwardHeader",
		// Forward host
		"-forwardHost", "host1",
	),
		Matches, OK)

	l, err := s.backend.GetLocation(h, loc)
	c.Assert(err, IsNil)
	c.Assert(l.Options.Timeouts.Read, Equals, "1s")
	c.Assert(l.Options.Timeouts.Dial, Equals, "2s")
	c.Assert(l.Options.Timeouts.TlsHandshake, Equals, "3s")

	c.Assert(l.Options.KeepAlive.Period, Equals, "4s")
	c.Assert(l.Options.KeepAlive.MaxIdleConnsPerHost, Equals, 5)

	c.Assert(l.Options.Limits.MaxMemBodyBytes, Equals, int64(6*1024))
	c.Assert(l.Options.Limits.MaxBodyBytes, Equals, int64(7*1024))

	c.Assert(l.Options.FailoverPredicate, Equals, "IsNetworkError")
	c.Assert(l.Options.TrustForwardHeader, Equals, true)
	c.Assert(l.Options.Hostname, Equals, "host1")
}

func (s *CmdSuite) TestLocationUpdateOptions(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, OK)

	h := "h"
	c.Assert(s.run("host", "add", "-name", h), Matches, OK)

	loc := "loc"
	path := "/path"
	c.Assert(s.run("location", "add", "-host", h, "-id", loc, "-up", up, "-path", path), Matches, OK)
	s.run("location", "set_options", "-host", h, "-id", loc, "-dialTimeout", "20s")

	l, err := s.backend.GetLocation(h, loc)
	c.Assert(err, IsNil)
	c.Assert(l.Options.Timeouts.Dial, Equals, "20s")
}
