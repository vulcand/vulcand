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
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/errors"
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
	ri, err := NewRewrite("^/foo(.*)", "$1", false, false)
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	out, err := ri.NewMiddleware()
	c.Assert(out, NotNil)
	c.Assert(err, IsNil)
}

func (s *RewriteSuite) TestNewRewriteBadParams(c *C) {
	// Bad regex
	_, err := NewRewriteInstance(&Rewrite{"[", "", false, false})
	c.Assert(err, NotNil)
}

func (s *RewriteSuite) TestNewRewriteFromOther(c *C) {
	ri, err := NewRewrite("^/foo(.*)", "$1", false, false)
	c.Assert(err, IsNil)

	r := Rewrite{"^/foo(.*)", "$1", false, false}

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

		rw := out.(*Rewrite)
		re, _ := regexp.Compile("^/foo(.*)")
		c.Assert(rw.Regexp, Equals, re.String())
		c.Assert(rw.Replacement, Equals, "$1")
		c.Assert(rw.RewriteBody, Equals, true)
		c.Assert(rw.Redirect, Equals, true)
	}
	app.Flags = CliFlags()
	app.Run([]string{"test", "--regexp=^/foo(.*)", "--replacement=$1", "--rewriteBody", "--redirect"})
	c.Assert(executed, Equals, true)
}

func (s *RewriteSuite) TestRewriteMatch(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.URL = &url.URL{Scheme: "http", Host: "localhost"}
	request.HttpRequest.RequestURI = "/foo/bar"

	ri, err := NewRewriteInstance(&Rewrite{"^http://localhost/foo(.*)", "http://localhost$1", false, false})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "http://localhost/bar")
}

func (s *RewriteSuite) TestRewriteNoMatch(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.URL = &url.URL{Scheme: "http", Host: "localhost"}
	request.HttpRequest.RequestURI = "/fooo/bar"

	ri, err := NewRewriteInstance(&Rewrite{"^http://localhost/foo/(.*)", "http://localhost/$1", false, false})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "http://localhost/fooo/bar")
}

func (s *RewriteSuite) TestRewriteSubstituteHeader(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "baz")
	request.HttpRequest.URL = &url.URL{Scheme: "http", Host: "localhost"}
	request.HttpRequest.RequestURI = "/foo/bar"

	ri, err := NewRewriteInstance(
		&Rewrite{"^http://localhost/(foo)/(bar)$", `http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2`, false, false})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "http://localhost/foo/baz/bar")
}

func (s *RewriteSuite) TestRewriteSubstituteMultipleHeaders(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "baz")
	request.HttpRequest.Header.Add("Y-Header", "bam")
	request.HttpRequest.URL = &url.URL{Scheme: "http", Host: "localhost"}
	request.HttpRequest.RequestURI = "/foo/bar"

	ri, err := NewRewriteInstance(
		&Rewrite{"^http://localhost/(foo)/(bar)$", `http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2/{{.Request.Header.Get "Y-Header"}}`, false, false})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "http://localhost/foo/baz/bar/bam")
}

func (s *RewriteSuite) TestRewriteSubstituteSameHeaderMultipleTimes(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "baz")
	request.HttpRequest.URL = &url.URL{Scheme: "http", Host: "localhost"}
	request.HttpRequest.RequestURI = "/foo/bar"

	ri, err := NewRewriteInstance(
		&Rewrite{"^http://localhost/(foo)/(bar)$", `http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2/{{.Request.Header.Get "X-Header"}}`, false, false})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "http://localhost/foo/baz/bar/baz")
}

func (s *RewriteSuite) TestRewriteSubstituteUnknownHeader(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.URL = &url.URL{Scheme: "http", Host: "localhost"}
	request.HttpRequest.RequestURI = "/foo/bar"

	ri, err := NewRewriteInstance(
		&Rewrite{"^http://localhost/(foo)/(bar)$", `http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2`, false, false})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "http://localhost/foo//bar")
}

func (s *RewriteSuite) TestRewriteUnknownVariable(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.URL = &url.URL{Scheme: "http", Host: "localhost"}
	request.HttpRequest.RequestURI = "/foo/bar"

	ri, err := NewRewriteInstance(&Rewrite{"^http://localhost/(foo)/(bar)$", "http://localhost/$1/{{.Unknown}}/$2", false, false})
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
	request.HttpRequest.URL = &url.URL{Scheme: "https", Host: "localhost"}
	request.HttpRequest.RequestURI = "/foo/bar"

	ri, err := NewRewriteInstance(&Rewrite{"^https://localhost/(foo)/(bar)$", "http://localhost/$1/$2", false, false})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)

	c.Assert(request.HttpRequest.URL.String(), Equals, "http://localhost/foo/bar")
}

func (s *RewriteSuite) TestRedirect(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "baz")
	request.HttpRequest.URL = &url.URL{Scheme: "http", Host: "localhost"}
	request.HttpRequest.RequestURI = "/foo/bar"

	ri, err := NewRewriteInstance(
		&Rewrite{"^http://localhost/(foo)/(bar)$", `http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2`, false, true})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	response, err := ri.ProcessRequest(request)
	c.Assert(response, IsNil)
	c.Assert(err, NotNil)

	redirectError, ok := err.(*errors.RedirectError)
	c.Assert(ok, Equals, true)
	c.Assert(redirectError.URL.String(), Equals, "http://localhost/foo/baz/bar")
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

	ri, err := NewRewriteInstance(&Rewrite{"", "", true, false})
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

	ri, err := NewRewriteInstance(&Rewrite{"", "", false, false})
	c.Assert(ri, NotNil)
	c.Assert(err, IsNil)

	ri.ProcessResponse(request, attempt)
	newBody, _ := ioutil.ReadAll(attempt.Response.Body)
	c.Assert(`{"foo": "{{.Request.Header.Get "X-Header"}}"}`, Equals, string(newBody))
}

func (s *RewriteSuite) TestRewriteCloseBody(c *C) {
	request := &BaseRequest{}
	request.HttpRequest = &http.Request{}
	request.HttpRequest.Header = make(http.Header)
	request.HttpRequest.Header.Add("X-Header", "bar")
	request.HttpRequest.URL, _ = url.Parse("http://foo")

	attempt := &BaseAttempt{}
	attempt.Response = &http.Response{}
	body := newTestCloser(strings.NewReader(`{"foo": "{{.Request.Header.Get "X-Header"}}"}`))
	attempt.Response.Body = body

	ri, _ := NewRewriteInstance(&Rewrite{"", "", true, false})
	ri.ProcessResponse(request, attempt)
	c.Assert(body.Closed, Equals, true)
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
