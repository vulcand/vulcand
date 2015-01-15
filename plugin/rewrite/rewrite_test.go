package rewrite

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/oxy/testutils"
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

	out, err := ri.NewHandler(nil)
	c.Assert(out, NotNil)
	c.Assert(err, IsNil)
}

func (s *RewriteSuite) TestNewRewriteBadParams(c *C) {
	// Bad regex
	_, err := newRewriteHandler(nil, &Rewrite{"[", "", false, false})
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

func (s *RewriteSuite) TestNewRewriteFromCLIOK(c *C) {
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
	var outURL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outURL = rawURL(req)
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler, &Rewrite{"^http://localhost/foo(.*)", "http://localhost$1", false, false})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/foo/bar", testutils.Host("localhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(outURL, Equals, "http://localhost/bar")
}

func (s *RewriteSuite) TestRewriteNoMatch(c *C) {
	var outURL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outURL = rawURL(req)
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler, &Rewrite{"^http://localhost/foo/(.*)", "http://localhost$1", false, false})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/fooo/bar", testutils.Host("localhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(outURL, Equals, "http://localhost/fooo/bar")
}

func (s *RewriteSuite) TestHeaderVar(c *C) {
	var outURL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outURL = rawURL(req)
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler,
		&Rewrite{"^http://localhost/(foo)/(bar)$", `http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2`, false, false})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/foo/bar", testutils.Host("localhost"), testutils.Header("X-Header", "baz"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(outURL, Equals, "http://localhost/foo/baz/bar")
}

func (s *RewriteSuite) TestMultipleHeaders(c *C) {
	var outURL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outURL = rawURL(req)
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler,
		&Rewrite{
			"^http://localhost/(foo)/(bar)$",
			`http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2/{{.Request.Header.Get "Y-Header"}}`, false, false})

	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/foo/bar",
		testutils.Host("localhost"), testutils.Header("X-Header", "baz"), testutils.Header("Y-Header", "bam"))

	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(outURL, Equals, "http://localhost/foo/baz/bar/bam")
}

func (s *RewriteSuite) TestSameHeaderMulti(c *C) {
	var outURL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outURL = rawURL(req)
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler,
		&Rewrite{
			"^http://localhost/(foo)/(bar)$",
			`http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2/{{.Request.Header.Get "X-Header"}}`, false, false})

	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/foo/bar",
		testutils.Host("localhost"), testutils.Header("X-Header", "baz"))

	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(outURL, Equals, "http://localhost/foo/baz/bar/baz")
}

func (s *RewriteSuite) TestUnknownHeader(c *C) {
	var outURL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outURL = rawURL(req)
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler,
		&Rewrite{"^http://localhost/(foo)/(bar)$", `http://localhost/$1/{{.Request.Header.Get "X-Header"}}/$2`, false, false})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/foo/bar", testutils.Host("localhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(outURL, Equals, "http://localhost/foo//bar")
}

func (s *RewriteSuite) TestUnknownVar(c *C) {
	var outURL string
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outURL = rawURL(req)
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler,
		&Rewrite{"^http://localhost/(foo)/(bar)$", `http://localhost/$1/{{.Bad}}/$2`, false, false})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/foo/bar", testutils.Host("localhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusInternalServerError)
}

// What real-world scenario does this test?
func (s *RewriteSuite) TestRewriteScheme(c *C) {
	var outURL *url.URL
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outURL = req.URL
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler, &Rewrite{"^https://localhost/(foo)/(bar)$", "http://localhost/$1/$2", false, false})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewUnstartedServer(rh)
	srv.StartTLS()
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/foo/bar", testutils.Host("localhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(outURL.Scheme, Equals, "http")
	c.Assert(outURL.Path, Equals, "/foo/bar")
	c.Assert(outURL.Host, Equals, "localhost")
}

func (s *RewriteSuite) TestRedirect(c *C) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("hello"))
	})

	rh, err := newRewriteHandler(handler, &Rewrite{"^http://localhost/(foo)/(bar)", "https://localhost/$2", false, true})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, _, err := testutils.Get(srv.URL+"/foo/bar", testutils.Host("localhost"))
	c.Assert(re.StatusCode, Equals, http.StatusFound)
	c.Assert(re.Header.Get("Location"), Equals, "https://localhost/bar")
}

func (s *RewriteSuite) TestRewriteResponseBody(c *C) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"foo": "{{.Request.Header.Get "X-Header"}}"}`))
	})

	rh, err := newRewriteHandler(handler, &Rewrite{"", "", true, false})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, body, err := testutils.Get(srv.URL,
		testutils.Host("localhost"),
		testutils.Header("X-Header", "bar"))

	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, `{"foo": "bar"}`)
}

func (s *RewriteSuite) TestDontRewriteResponseBody(c *C) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"foo": "{{.Request.Header.Get "X-Header"}}"}`))
	})

	rh, err := newRewriteHandler(handler, &Rewrite{"", "", false, false})
	c.Assert(rh, NotNil)
	c.Assert(err, IsNil)

	srv := httptest.NewServer(rh)
	defer srv.Close()

	re, body, err := testutils.Get(srv.URL,
		testutils.Host("localhost"),
		testutils.Header("X-Header", "bar"))

	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, `{"foo": "{{.Request.Header.Get "X-Header"}}"}`)
}
