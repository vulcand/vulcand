package netutils

import (
	"net/http"
	"net/url"
	"testing"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestUtils(t *testing.T) { TestingT(t) }

type NetUtilsSuite struct{}

var _ = Suite(&NetUtilsSuite{})

// Make sure parseUrl is strict enough not to accept total garbage
func (s *NetUtilsSuite) TestParseBadUrl(c *C) {
	badUrls := []string{
		"",
		" some random text ",
		"http---{}{\\bad bad url",
	}
	for _, badUrl := range badUrls {
		_, err := ParseUrl(badUrl)
		c.Assert(err, NotNil)
	}
}

// Make sure parseUrl is strict enough not to accept total garbage
func (s *NetUtilsSuite) TestURLRawPath(c *C) {
	vals := []struct {
		URL      string
		Expected string
	}{
		{"http://google.com/", "/"},
		{"http://google.com/a?q=b", "/a"},
		{"http://google.com/%2Fvalue/hello", "/%2Fvalue/hello"},
		{"/home", "/home"},
		{"/home?a=b", "/home"},
		{"/home%2F", "/home%2F"},
	}
	for _, v := range vals {
		out, err := RawPath(v.URL)
		c.Assert(err, IsNil)
		c.Assert(out, Equals, v.Expected)
	}
}

func (s *NetUtilsSuite) TestRawURL(c *C) {
	request := &http.Request{URL: &url.URL{Scheme: "http", Host: "localhost:8080"}, RequestURI: "/foo/bar"}
	c.Assert("http://localhost:8080/foo/bar", Equals, RawURL(request))
}

//Just to make sure we don't panic, return err and not
//username and pass and cover the function
func (s *NetUtilsSuite) TestParseBadHeaders(c *C) {
	headers := []string{
		//just empty string
		"",
		//missing auth type
		"justplainstring",
		//unknown auth type
		"Whut justplainstring",
		//invalid base64
		"Basic Shmasic",
		//random encoded string
		"Basic YW55IGNhcm5hbCBwbGVhcw==",
	}
	for _, h := range headers {
		_, err := ParseAuthHeader(h)
		c.Assert(err, NotNil)
	}
}

//Just to make sure we don't panic, return err and not
//username and pass and cover the function
func (s *NetUtilsSuite) TestParseSuccess(c *C) {
	headers := []struct {
		Header   string
		Expected BasicAuth
	}{
		{
			"Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
			BasicAuth{Username: "Aladdin", Password: "open sesame"},
		},
		// Make sure that String() produces valid header
		{
			(&BasicAuth{Username: "Alice", Password: "Here's bob"}).String(),
			BasicAuth{Username: "Alice", Password: "Here's bob"},
		},
		//empty pass
		{
			"Basic QWxhZGRpbjo=",
			BasicAuth{Username: "Aladdin", Password: ""},
		},
	}
	for _, h := range headers {
		request, err := ParseAuthHeader(h.Header)
		c.Assert(err, IsNil)
		c.Assert(request.Username, Equals, h.Expected.Username)
		c.Assert(request.Password, Equals, h.Expected.Password)

	}
}

// Make sure copy does it right, so the copied url
// is safe to alter without modifying the other
func (s *NetUtilsSuite) TestCopyUrl(c *C) {
	urlA := &url.URL{
		Scheme:   "http",
		Host:     "localhost:5000",
		Path:     "/upstream",
		Opaque:   "opaque",
		RawQuery: "a=1&b=2",
		Fragment: "#hello",
		User:     &url.Userinfo{},
	}
	urlB := CopyUrl(urlA)
	c.Assert(urlB, DeepEquals, urlB)
	urlB.Scheme = "https"
	c.Assert(urlB, Not(DeepEquals), urlA)
}

// Make sure copy headers is not shallow and copies all headers
func (s *NetUtilsSuite) TestCopyHeaders(c *C) {
	source, destination := make(http.Header), make(http.Header)
	source.Add("a", "b")
	source.Add("c", "d")

	CopyHeaders(destination, source)

	c.Assert(destination.Get("a"), Equals, "b")
	c.Assert(destination.Get("c"), Equals, "d")

	// make sure that altering source does not affect the destination
	source.Del("a")
	c.Assert(source.Get("a"), Equals, "")
	c.Assert(destination.Get("a"), Equals, "b")
}

func (s *NetUtilsSuite) TestHasHeaders(c *C) {
	source := make(http.Header)
	source.Add("a", "b")
	source.Add("c", "d")
	c.Assert(HasHeaders([]string{"a", "f"}, source), Equals, true)
	c.Assert(HasHeaders([]string{"i", "j"}, source), Equals, false)
}

func (s *NetUtilsSuite) TestRemoveHeaders(c *C) {
	source := make(http.Header)
	source.Add("a", "b")
	source.Add("a", "m")
	source.Add("c", "d")
	RemoveHeaders([]string{"a"}, source)
	c.Assert(source.Get("a"), Equals, "")
	c.Assert(source.Get("c"), Equals, "d")
}
