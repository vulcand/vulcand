package vulcan

import (
	"encoding/json"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll/vulcan/middleware"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestR(t *testing.T) { TestingT(t) }

type RSuite struct{}

var _ = Suite(&RSuite{})

func (s *RSuite) TestRoute(c *C) {
	l := NewLocation("localhost", []string{"GET"}, "/hello/{world}", "b1", []middleware.Middleware{})
	c.Assert(l.Route(), Equals, `Host("localhost") && Method("GET") && Path("/hello/<world>")`)

	l = NewLocation("localhost", []string{"GET", "POST"}, "/hello/{world}", "b1", []middleware.Middleware{})
	c.Assert(l.Route(), Equals, `Host("localhost") && MethodRegexp("GET|POST") && Path("/hello/<world>")`)
}

func (s *RSuite) TestSpec(c *C) {
	l := NewLocation("localhost", []string{"GET"}, "/hello/{world}", "b1", []middleware.Middleware{})
	spec, err := l.Spec()
	c.Assert(err, IsNil)

	var out map[string]interface{}
	c.Assert(json.Unmarshal([]byte(spec), &out), IsNil)
}

func (s *RSuite) TestServer(c *C) {
	e, err := NewEndpoint("backend", "127.0.0.1", 8000)
	c.Assert(err, IsNil)

	spec, err := e.BackendSpec()
	c.Assert(err, IsNil)
	var out map[string]interface{}
	c.Assert(json.Unmarshal([]byte(spec), &out), IsNil)

	spec, err = e.ServerSpec()
	c.Assert(err, IsNil)
	var sout map[string]interface{}
	c.Assert(json.Unmarshal([]byte(spec), &sout), IsNil)
}
