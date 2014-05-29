package main

import (
	"bytes"
	"fmt"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/testutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/launchpad.net/gocheck"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestVulcanCommandLineTool(t *testing.T) { TestingT(t) }

type CmdSuite struct {
	vulcanApiUrl     string
	vulcanServiceUrl string
	out              *bytes.Buffer
	cmd              *Command
}

var _ = Suite(&CmdSuite{})

func (s *CmdSuite) name(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
}

func (s *CmdSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	vulcanApiUrl := os.Getenv("VULCAND_TEST_API_URL")
	if vulcanApiUrl == "" {
		// Skips the entire suite
		c.Skip("This test requires running vulcand daemon, provide API url via VULCAND_TEST_API_URL environment variable")
		return
	}
	s.vulcanApiUrl = vulcanApiUrl

	vulcanServiceUrl := os.Getenv("VULCAND_TEST_SERVICE_URL")
	if vulcanServiceUrl == "" {
		// Skips the entire suite
		c.Skip("This test requires running vulcand daemon, provide API url via VULCAND_TEST_SERVICE_URL environment variable")
		return
	}
	s.vulcanServiceUrl = vulcanServiceUrl
}

func (s *CmdSuite) runString(in string) string {
	return s.run(strings.Split(in, " ")...)
}

func (s *CmdSuite) run(params ...string) string {
	args := []string{"vulcanctl"}
	args = append(args, params...)
	args = append(args, fmt.Sprintf("--vulcan=%s", s.vulcanApiUrl))
	s.cmd.Run(args)
	return strings.Replace(s.out.String(), "\n", " ", -1)
}

func (s *CmdSuite) SetUpTest(c *C) {
	s.out = &bytes.Buffer{}
	s.cmd = &Command{out: s.out, vulcanUrl: s.vulcanServiceUrl}
}

func (s *CmdSuite) TestStatus(c *C) {
	c.Assert(s.run("status"), Matches, ".*hosts.*")
}

func (s *CmdSuite) TestHostCrud(c *C) {
	host := s.name("host")
	c.Assert(s.run("host", "add", "-name", host), Matches, ".*added.*")
	c.Assert(s.run("host", "rm", "-name", host), Matches, ".*deleted.*")
}

func (s *CmdSuite) TestUpstreamCrud(c *C) {
	up := s.name("up")
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, ".*deleted.*")
	c.Assert(s.run("upstream", "ls"), Matches, fmt.Sprintf(".*%s.*", up))
}

func (s *CmdSuite) TestUpstreamAutoId(c *C) {
	c.Assert(s.run("upstream", "add"), Matches, ".*added.*")
}

func (s *CmdSuite) TestEndpointCrud(c *C) {
	up := s.name("up")
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")
	e := s.name("e")
	c.Assert(s.run("endpoint", "add", "-id", e, "-url", "http://localhost:5000", "-up", up), Matches, ".*added.*")

	c.Assert(s.run("endpoint", "rm", "-id", e, "-up", up), Matches, ".*deleted.*")
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, ".*deleted.*")
}

func (s *CmdSuite) TestLimitsCrud(c *C) {
	// Create upstream with this location
	up := s.name("up")
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")

	h := s.name("h")
	c.Assert(s.run("host", "add", "-name", h), Matches, ".*added.*")

	loc := s.name("loc")
	path := s.name("/path")
	c.Assert(s.run("location", "add", "-host", h, "-id", loc, "-up", up, "-path", path), Matches, ".*added.*")

	rl := s.name("rl")
	c.Assert(s.run("ratelimit", "add", "-host", h, "-loc", loc, "-id", rl, "-requests", "10", "-variable", "client.ip", "-period", "3"), Matches, ".*added.*")
	c.Assert(s.run("ratelimit", "update", "-host", h, "-loc", loc, "-id", rl, "-requests", "100", "-variable", "client.ip", "-period", "30"), Matches, ".*updated.*")
	c.Assert(s.run("ratelimit", "rm", "-host", h, "-loc", loc, "-id", rl), Matches, ".*deleted.*")

	cl := s.name("cl")
	c.Assert(s.run("connlimit", "add", "-host", h, "-loc", loc, "-id", cl, "-connections", "10", "-variable", "client.ip"), Matches, ".*added.*")
	c.Assert(s.run("connlimit", "update", "-host", h, "-loc", loc, "-id", cl, "-connections", "100", "-variable", "client.ip"), Matches, ".*updated.*")
	c.Assert(s.run("connlimit", "rm", "-host", h, "-loc", loc, "-id", cl), Matches, ".*deleted.*")

	c.Assert(s.run("location", "rm", "-host", h, "-id", loc), Matches, ".*deleted.*")
	c.Assert(s.run("host", "rm", "-name", h), Matches, ".*deleted.*")
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, ".*deleted.*")
}

// Set up a location with a path, hit this location and make sure everything worked fine
func (s *CmdSuite) TestLocationCrud(c *C) {
	called := false
	server := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("Hi, I'm fine, thanks!"))
	})
	defer server.Close()

	// Create upstream with this location
	up := s.name("up")
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")
	e := s.name("e")
	c.Assert(s.run("endpoint", "add", "-id", e, "-url", server.URL, "-up", up), Matches, ".*added.*")

	// Add localhost, we don't care if it already exists
	s.run("host", "add", "-name", "localhost")

	loc := s.name("loc")
	path := s.name("/path")

	c.Assert(s.run("location", "add", "-host", "localhost", "-id", loc, "-up", up, "-path", path), Matches, ".*added.*")

	time.Sleep(time.Second)
	url := fmt.Sprintf("%s%s", s.vulcanServiceUrl, path)
	response, _ := Get(c, url, nil, "")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(called, Equals, true)
}

func (s *CmdSuite) TestUpstreamDrainConnections(c *C) {
	up := s.name("up")
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")
	c.Assert(s.run("upstream", "drain", "--id", up, "--timeout", "0"), Matches, ".*Connections: 0.*")
}

func (s *CmdSuite) TestLocationUpdateUpstream(c *C) {
	re1 := s.name("1")
	server1 := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(re1))
	})
	defer server1.Close()

	re2 := s.name("2")
	server2 := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(re2))
	})
	defer server2.Close()

	up1 := s.name("up1")
	c.Assert(s.run("upstream", "add", "-id", up1), Matches, ".*added.*")
	c.Assert(s.run("endpoint", "add", "-url", server1.URL, "-up", up1), Matches, ".*added.*")

	up2 := s.name("up2")
	c.Assert(s.run("upstream", "add", "-id", up2), Matches, ".*added.*")
	c.Assert(s.run("endpoint", "add", "-url", server2.URL, "-up", up2), Matches, ".*added.*")

	// Add localhost, we don't care if it already exists
	s.run("host", "add", "-name", "localhost")

	loc := s.name("loc")
	path := s.name("/path")

	c.Assert(s.run("location", "add", "-host", "localhost", "-id", loc, "-up", up1, "-path", path), Matches, ".*added.*")

	time.Sleep(time.Second)
	url := fmt.Sprintf("%s%s", s.vulcanServiceUrl, path)
	response, body := Get(c, url, nil, "")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, re1)

	c.Assert(s.run("location", "set_upstream", "-host", "localhost", "-id", loc, "-up", up2), Matches, ".*updated.*")

	time.Sleep(time.Second)
	response, body = Get(c, url, nil, "")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, re2)
}
