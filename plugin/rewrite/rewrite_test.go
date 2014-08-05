package rewrite

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/plugin"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestRL(t *testing.T) { TestingT(t) }

type RewriteSuite struct {
}

var _ = Suite(&RewriteSuite{})

// One of the most important tests:
// Make sure the Rewrite spec is compatible and will be accepted by middleware registry
func (s *RewriteSuite) TestSpecIsOK(c *C) {
	c.Assert(plugin.NewRegistry().AddSpec(GetSpec()), IsNil)
}

func (s *RewriteSuite) TestNewRewriteSuccess(c *C) {
	ri, err := NewRewriteInstance("^/foo(.*)", "$1")
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	out, err := ri.NewMiddleware()
	c.Assert(out, NotNil)
	c.Assert(err, IsNil)
}

func (s *RewriteSuite) TestNewRewriteBadParams(c *C) {
	// Bad regex
	_, err := NewRewriteInstance("[", "")
	c.Assert(err, NotNil)
}

func (s *RewriteSuite) TestNewRewriteFromOther(c *C) {
	ri, err := NewRewriteInstance("^/foo(.*)", "$1")
	c.Assert(err, IsNil)

	r := Rewrite{"^/foo(.*)", "$1"}

	out, err := FromOther(r)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ri)
}

func (s *RewriteSuite) TestNewRewriteFromCliOk(c *C) {
	app := cli.NewApp()
	app.Name = "test"
	executed := false
	app.Action = func(ctx *cli.Context) {
		executed = true
		out, err := FromCli(ctx)
		c.Assert(out, NotNil)
		c.Assert(err, IsNil)

		ri := out.(*RewriteInstance)
		re, _ := regexp.Compile("^/foo(.*)")
		c.Assert(ri.regexp.String(), Equals, re.String())
		c.Assert(ri.replacement, Equals, "$1")
	}
	app.Flags = CliFlags()
	app.Run([]string{"test", "--regexp=^/foo(.*)", "--replacement=$1"})
	c.Assert(executed, Equals, true)
}

func (s *RewriteSuite) TestRewriteMatch(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/foo/bar"

	ri, err := NewRewriteInstance("^/foo(.*)", "$1")
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(string(ri.newPath), Equals, "/bar")
}

func (s *RewriteSuite) TestRewriteNoMatch(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/fooo/bar"

	ri, err := NewRewriteInstance("^/foo/(.*)", "/$1")
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(string(ri.newPath), Equals, "/fooo/bar")
}
