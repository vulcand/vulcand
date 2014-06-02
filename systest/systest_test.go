// This package contains "Black box" tests
// That configure Vulcand using various methods and making sure
// Vulcand acts accorgindly - e.g. is capable of servicing requests
package systest

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/testutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/launchpad.net/gocheck"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestVulcandWithEtcd(t *testing.T) { TestingT(t) }

// Performs "Black box" system test of Vulcan backed by Etcd
// By talking directly to Etcd and executing commands back
type VESuite struct {
	apiUrl     string
	serviceUrl string
	etcdNodes  string
	etcdPrefix string
	client     *etcd.Client
}

var _ = Suite(&VESuite{})

func (s *VESuite) name(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
}

func (s *VESuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	s.etcdNodes = os.Getenv("VULCAND_TEST_ETCD_NODES")
	if s.etcdNodes == "" {
		c.Skip("This test requires running Etcd, please provide url via VULCAND_TEST_ETCD_NODES environment variable")
		return
	}
	s.client = etcd.NewClient(strings.Split(s.etcdNodes, ","))

	s.etcdPrefix = os.Getenv("VULCAND_TEST_ETCD_PREFIX")
	if s.etcdPrefix == "" {
		c.Skip("This test requires Etcd prefix, please provide url via VULCAND_TEST_ETCD_PREFIX environment variable")
		return
	}

	s.apiUrl = os.Getenv("VULCAND_TEST_API_URL")
	if s.apiUrl == "" {
		c.Skip("This test requires running vulcand daemon, provide API url via VULCAND_TEST_API_URL environment variable")
		return
	}

	s.serviceUrl = os.Getenv("VULCAND_TEST_SERVICE_URL")
	if s.serviceUrl == "" {
		c.Skip("This test requires running vulcand daemon, provide API url via VULCAND_TEST_SERVICE_URL environment variable")
		return
	}
}

func (s VESuite) path(keys ...string) string {
	return strings.Join(append([]string{s.etcdPrefix}, keys...), "/")
}

func (s *VESuite) SetUpTest(c *C) {
	// Delete all values under the given prefix
	_, err := s.client.Get(s.etcdPrefix, false, false)
	if err != nil {
		e, ok := err.(*etcd.EtcdError)
		// We haven't expected this error, oops
		if !ok || e.ErrorCode != 100 {
			c.Assert(err, IsNil)
		}
	} else {
		_, err = s.client.Delete(s.etcdPrefix, true)
		c.Assert(err, IsNil)
	}
}

// Set up a location with a path, hit this location and make sure everything worked fine
func (s *VESuite) TestLocationCrud(c *C) {
	called := false
	server := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("Hi, I'm fine, thanks!"))
	})
	defer server.Close()

	// Create upstream and endpoint
	up, e, url := "up1", "e1", server.URL
	_, err := s.client.Set(s.path("upstreams", up, "endpoints", e), url, 0)
	c.Assert(err, IsNil)

	// Add location
	host, locId, path := "localhost", "loc1", "/path"
	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "path"), path, 0)
	c.Assert(err, IsNil)
	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "upstream"), up, 0)
	c.Assert(err, IsNil)

	time.Sleep(time.Second)
	response, _ := Get(c, fmt.Sprintf("%s%s", s.serviceUrl, path), nil, "")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(called, Equals, true)
}

func (s *VESuite) TestLocationCreateUpstreamFirst(c *C) {
	called := false
	server := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("Hi, I'm fine, thanks!"))
	})
	defer server.Close()

	// Create upstream and endpoint
	up, e, url := "up1", "e1", server.URL
	_, err := s.client.Set(s.path("upstreams", up, "endpoints", e), url, 0)
	c.Assert(err, IsNil)

	// Add location
	host, locId, path := "localhost", "loc1", "/path"
	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "upstream"), up, 0)
	c.Assert(err, IsNil)
	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "path"), path, 0)
	c.Assert(err, IsNil)

	time.Sleep(time.Second)
	response, _ := Get(c, fmt.Sprintf("%s%s", s.serviceUrl, path), nil, "")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(called, Equals, true)
}

func (s *VESuite) TestUpdateUpstreamLocation(c *C) {
	server1 := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("1"))
	})
	defer server1.Close()

	server2 := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("2"))
	})
	defer server2.Close()

	// Create two upstreams and endpoints
	up1, e1, url1 := "up1", "e1", server1.URL
	_, err := s.client.Set(s.path("upstreams", up1, "endpoints", e1), url1, 0)
	c.Assert(err, IsNil)

	up2, e2, url2 := "up2", "e2", server2.URL
	_, err = s.client.Set(s.path("upstreams", up2, "endpoints", e2), url2, 0)
	c.Assert(err, IsNil)

	// Add location, intitally pointing to the first upstream
	host, locId, path := "localhost", "loc1", "/path"
	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "path"), path, 0)
	c.Assert(err, IsNil)
	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "upstream"), up1, 0)
	c.Assert(err, IsNil)

	time.Sleep(time.Second)
	url := fmt.Sprintf("%s%s", s.serviceUrl, path)
	response, body := Get(c, url, nil, "")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, "1")

	// Update the upstream
	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "upstream"), up2, 0)
	c.Assert(err, IsNil)

	time.Sleep(time.Second)
	response, body = Get(c, url, nil, "")
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, "2")
}
