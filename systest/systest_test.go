// This package contains "Black box" tests
// That configure Vulcand using various methods and making sure
// Vulcand acts accorgindly - e.g. is capable of servicing requests
package systest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/testutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-etcd/etcd"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/secret"
	. "github.com/mailgun/vulcand/testutils"
)

func TestVulcandWithEtcd(t *testing.T) { TestingT(t) }

// Performs "Black box" system test of Vulcan backed by Etcd
// By talking directly to Etcd and executing commands back
type VESuite struct {
	apiUrl     string
	serviceUrl string
	etcdNodes  string
	etcdPrefix string
	sealKey    string
	box        *secret.Box
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

	s.sealKey = os.Getenv("VULCAND_TEST_SEAL_KEY")
	if s.sealKey == "" {
		c.Skip("This test requires running vulcand daemon, provide API url via VULCAND_TEST_SEAL_KEY environment variable")
		return
	}

	key, err := secret.KeyFromString(s.sealKey)
	c.Assert(err, IsNil)

	box, err := secret.NewBox(key)
	c.Assert(err, IsNil)

	s.box = box
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
	response, _, err := GET(fmt.Sprintf("%s%s", s.serviceUrl, path), Opts{})
	c.Assert(err, IsNil)
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
	response, _, err := GET(fmt.Sprintf("%s%s", s.serviceUrl, path), Opts{})
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(called, Equals, true)
}

func (s *VESuite) TestLocationUpdateLimits(c *C) {
	var headers http.Header
	server := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header
		w.Write([]byte("Hello, I'm totally fine"))
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
	response, _, err := GET(fmt.Sprintf("%s%s", s.serviceUrl, path), Opts{})
	c.Assert(err, IsNil)

	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(response.Header.Get("X-Forwarded-For"), Not(Equals), "hello")

	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "options"), `{"Limits": {"MaxMemBodyBytes":2, "MaxBodyBytes":4}}`, 0)
	c.Assert(err, IsNil)
	time.Sleep(time.Second)

	response, _, err = GET(fmt.Sprintf("%s%s", s.serviceUrl, path), Opts{Body: "This is longer than allowed 4 bytes"})
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusRequestEntityTooLarge)
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
	response, body, err := GET(url, Opts{})
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, "1")

	// Update the upstream
	_, err = s.client.Set(s.path("hosts", host, "locations", locId, "upstream"), up2, 0)
	c.Assert(err, IsNil)

	time.Sleep(time.Second)
	response, body, err = GET(url, Opts{})
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, "2")
}

// Set up a location with a path, hit this location and make sure everything worked fine
func (s *VESuite) TestHTTPListenerCrud(c *C) {
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

	// Add HTTP listener
	l1 := "l1"
	listener, err := backend.NewListener(l1, "http", "tcp", "localhost:31000")
	c.Assert(err, IsNil)
	bytes, err := json.Marshal(listener)
	c.Assert(err, IsNil)
	s.client.Set(s.path("hosts", host, "listeners", l1), string(bytes), 0)

	time.Sleep(time.Second)
	_, _, err = GET(fmt.Sprintf("%s%s", "http://localhost:31000", path), Opts{})
	c.Assert(err, IsNil)
	c.Assert(called, Equals, true)

	_, err = s.client.Delete(s.path("hosts", host, "listeners", l1), true)
	c.Assert(err, IsNil)

	time.Sleep(time.Second)

	_, _, err = GET(fmt.Sprintf("%s%s", "http://localhost:31000", path), Opts{})
	c.Assert(err, NotNil)
}

func (s *VESuite) TestHTTPSListenerCrud(c *C) {
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

	keyPair := NewTestKeyPair()

	bytes, err := secret.SealKeyPairToJSON(s.box, keyPair)
	c.Assert(err, IsNil)

	_, err = s.client.Set(s.path("hosts", host, "keypair"), string(bytes), 0)
	c.Assert(err, IsNil)

	// Add HTTPS listener
	l := "l2"
	listener, err := backend.NewListener(l, "https", "tcp", "localhost:32000")
	c.Assert(err, IsNil)
	bytes, err = json.Marshal(listener)
	c.Assert(err, IsNil)
	s.client.Set(s.path("hosts", host, "listeners", l), string(bytes), 0)

	time.Sleep(time.Second)
	_, _, err = GET(fmt.Sprintf("%s%s", "https://localhost:32000", path), Opts{})
	c.Assert(err, IsNil)
	c.Assert(called, Equals, true)

	_, err = s.client.Delete(s.path("hosts", host, "listeners", l), true)
	c.Assert(err, IsNil)

	time.Sleep(time.Second)

	_, _, err = GET(fmt.Sprintf("%s%s", "https://localhost:32000", path), Opts{})
	c.Assert(err, NotNil)
}
