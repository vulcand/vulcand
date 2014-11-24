package rewrite

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"github.com/mailgun/vulcand/plugin"
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
	ri, err := NewRewriteInstance("^/foo(.*)", "$1", false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	out, err := ri.NewMiddleware()
	c.Assert(out, NotNil)
	c.Assert(err, IsNil)
}

func (s *RewriteSuite) TestNewRewriteBadParams(c *C) {
	// Bad regex
	_, err := NewRewriteInstance("[", "", false)
	c.Assert(err, NotNil)
}

func (s *RewriteSuite) TestNewRewriteFromOther(c *C) {
	ri, err := NewRewriteInstance("^/foo(.*)", "$1", false)
	c.Assert(err, IsNil)

	r := Rewrite{"^/foo(.*)", "$1", false}

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
		c.Assert(ri.rewriteBody, Equals, true)
	}
	app.Flags = CliFlags()
	app.Run([]string{"test", "--regexp=^/foo(.*)", "--replacement=$1", "--rewriteBody"})
	c.Assert(executed, Equals, true)
}

func (s *RewriteSuite) TestRewriteMatch(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/foo/bar"

	ri, err := NewRewriteInstance("^/foo(.*)", "$1", false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "/bar")
}

func (s *RewriteSuite) TestRewriteNoMatch(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/fooo/bar"

	ri, err := NewRewriteInstance("^/foo/(.*)", "/$1", false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "/fooo/bar")
}

func (s *RewriteSuite) TestRewriteSubstituteHeader(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "baz")
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/foo/bar"

	ri, err := NewRewriteInstance("^/(foo)/(bar)$", `/$1/{{.Request.Header.Get "X-Header"}}/$2`, false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "/foo/baz/bar")
}

func (s *RewriteSuite) TestRewriteSubstituteMultipleHeaders(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "baz")
	request.HttpRequest.Header.Add("Y-Header", "bam")
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/foo/bar"

	ri, err := NewRewriteInstance(
		"^/(foo)/(bar)$", `/$1/{{.Request.Header.Get "X-Header"}}/$2/{{.Request.Header.Get "Y-Header"}}`, false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "/foo/baz/bar/bam")
}

func (s *RewriteSuite) TestRewriteSubstituteSameHeaderMultipleTimes(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "baz")
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/foo/bar"

	ri, err := NewRewriteInstance(
		"^/(foo)/(bar)$", `/$1/{{.Request.Header.Get "X-Header"}}/$2/{{.Request.Header.Get "X-Header"}}`, false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "/foo/baz/bar/baz")
}

func (s *RewriteSuite) TestRewriteSubstituteUnknownHeader(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/foo/bar"

	ri, err := NewRewriteInstance("^/(foo)/(bar)$", `/$1/{{.Request.Header.Get "X-Header"}}/$2`, false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "/foo//bar")
}

func (s *RewriteSuite) TestRewriteUnknownVariable(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.URL = &url.URL{}
	request.HttpRequest.URL.Path = "/foo/bar"

	ri, err := NewRewriteInstance("^/(foo)/(bar)$", "/$1/{{.Unknown}}/$2", false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, NotNil)
}

func (s *RewriteSuite) TestRewriteHTTPSToHTTP(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.URL, _ = url.Parse("https://foo/bar")

	ri, err := NewRewriteInstance("^https://(foo)/(bar)$", "http://$1/$2", false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "http://foo/bar")
}

func (s *RewriteSuite) TestRewriteResponseBody(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "bar")
	request.HttpRequest.URL, _ = url.Parse("http://foo")

	attempt := &BaseAttempt{}
	attempt.Response = &http.Response{}
	attempt.Response.Body = ioutil.NopCloser(strings.NewReader(`{"foo": "{{.Request.Header.Get "X-Header"}}"}`))

	ri, err := NewRewriteInstance("", "", true)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	ri.ProcessResponse(request, attempt)
	newBody, _ := ioutil.ReadAll(attempt.Response.Body)
	c.Assert(`{"foo": "bar"}`, Equals, string(newBody))
}

func (s *RewriteSuite) TestNotRewriteResponseBody(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "bar")
	request.HttpRequest.URL, _ = url.Parse("http://foo")

	attempt := &BaseAttempt{}
	attempt.Response = &http.Response{}
	attempt.Response.Body = ioutil.NopCloser(strings.NewReader(`{"foo": "{{.Request.Header.Get "X-Header"}}"}`))

	ri, err := NewRewriteInstance("", "", false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	ri.ProcessResponse(request, attempt)
	newBody, _ := ioutil.ReadAll(attempt.Response.Body)
	c.Assert(`{"foo": "{{.Request.Header.Get "X-Header"}}"}`, Equals, string(newBody))
}

// Verify that if templating succeeds, the old body is closed.
func (s *RewriteSuite) TestRewriteTemplateSuccessCloseBody(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "bar")
	request.HttpRequest.URL, _ = url.Parse("http://foo")

	attempt := &BaseAttempt{}
	attempt.Response = &http.Response{}
	body := newTestCloser(strings.NewReader(`{"foo": "{{.Request.Header.Get "X-Header"}}"}`))
	attempt.Response.Body = body

	ri, _ := NewRewriteInstance("", "", true)
	ri.ProcessResponse(request, attempt)
	c.Assert(body.Closed, Equals, true)
}

// Verify that if templating fails, the old body remains open.
func (s *RewriteSuite) TestRewriteTemplateFailNotCloseBody(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "bar")
	request.HttpRequest.URL, _ = url.Parse("http://foo")

	attempt := &BaseAttempt{}
	attempt.Response = &http.Response{}
	body := newTestCloser(strings.NewReader(`{"foo": "{{.Request.Header.Get "X-Header""}`))
	attempt.Response.Body = body

	ri, _ := NewRewriteInstance("", "", true)
	ri.ProcessResponse(request, attempt)
	c.Assert(body.Closed, Equals, false)
}

// testCloser is a ReadCloser that allows to learn whether it was closed.
type testCloser struct {
	io.Reader
	Closed bool
}

func newTestCloser(r io.Reader) *testCloser {
	return &testCloser{r, false}
}

func (c *testCloser) Close() error {
	c.Closed = true
	return nil
}
