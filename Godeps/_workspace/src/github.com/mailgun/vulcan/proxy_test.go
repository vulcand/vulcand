package vulcan

import (
	"github.com/mailgun/timetools"
	. "github.com/mailgun/vulcan/location"
	. "github.com/mailgun/vulcan/route"
	. "github.com/mailgun/vulcan/testutils"
	. "gopkg.in/check.v1"
	"net/http"
	"net/http/httptest"
	"time"
)

type ProxySuite struct {
	authHeaders http.Header
	tm          *timetools.FreezedTime
}

var _ = Suite(&ProxySuite{
	tm: &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	},
})

// Success, make sure we've successfully proxied the response
func (s *ProxySuite) TestSuccess(c *C) {
	server := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint"))
	})
	defer server.Close()

	proxy, err := NewProxy(&ConstRouter{&ConstHttpLocation{server.URL}})
	c.Assert(err, IsNil)
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	response, bodyBytes, err := MakeRequest(proxyServer.URL, Opts{})
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint")
}

func (s *ProxySuite) TestFailure(c *C) {
	proxy, err := NewProxy(&ConstRouter{&ConstHttpLocation{"http://localhost:63999"}})
	c.Assert(err, IsNil)
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	response, _, err := MakeRequest(proxyServer.URL, Opts{})
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusBadGateway)
}

func (s *ProxySuite) TestReadTimeout(c *C) {
	c.Skip("This test is not stable")

	server := NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi, I'm endpoint"))
	})
	defer server.Close()

	proxy, err := NewProxy(&ConstRouter{&ConstHttpLocation{server.URL}})
	c.Assert(err, IsNil)

	// Set a very short read timeout
	proxyServer := httptest.NewUnstartedServer(proxy)
	proxyServer.Config.ReadTimeout = time.Millisecond
	proxyServer.Start()
	defer proxyServer.Close()

	value := make([]byte, 65636)
	for i := 0; i < len(value); i += 1 {
		value[i] = byte(i % 255)
	}

	response, _, err := MakeRequest(proxyServer.URL, Opts{Body: string(value)})
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusRequestTimeout)
}
