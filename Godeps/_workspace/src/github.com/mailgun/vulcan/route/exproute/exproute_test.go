package exproute

import (
	"net/http"
	"testing"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

func TestRoute(t *testing.T) { TestingT(t) }

type RouteSuite struct {
}

var _ = Suite(&RouteSuite{})

func (s *RouteSuite) TestConvertPath(c *C) {
	tc := []struct {
		in  string
		out string
	}{
		{"/hello", `PathRegexp("/hello")`},
		{`TrieRoute("/hello")`, `Path("/hello")`},
		{`TrieRoute("POST", "/hello")`, `Method("POST") && Path("/hello")`},
		{`TrieRoute("POST", "PUT", "/v2/path")`, `MethodRegexp("POST|PUT") && Path("/v2/path")`},
		{`RegexpRoute("/hello")`, `PathRegexp("/hello")`},
		{`RegexpRoute("POST", "/hello")`, `Method("POST") && PathRegexp("/hello")`},
		{`RegexpRoute("POST", "PUT", "/v2/path")`, `MethodRegexp("POST|PUT") && PathRegexp("/v2/path")`},
		{`Path("/hello")`, `Path("/hello")`},
	}
	for i, t := range tc {
		comment := Commentf("tc%d", i)
		c.Assert(convertPath(t.in), Equals, t.out, comment)
	}
}

func (s *RouteSuite) TestEmptyOperationsSucceed(c *C) {
	r := NewExpRouter()

	c.Assert(r.GetLocationByExpression("bla"), IsNil)
	c.Assert(r.RemoveLocationByExpression("bla"), IsNil)

	l, err := r.Route(makeReq("http://google.com/blabla"))
	c.Assert(err, IsNil)
	c.Assert(l, IsNil)
}

func (s *RouteSuite) TestCRUD(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`TrieRoute("/r1")`, l1), IsNil)
	c.Assert(r.GetLocationByExpression(`TrieRoute("/r1")`), Equals, l1)
	c.Assert(r.RemoveLocationByExpression(`TrieRoute("/r1")`), IsNil)
	c.Assert(r.GetLocationByExpression(`TrieRoute("/r1")`), IsNil)
}

func (s *RouteSuite) TestAddTwiceFails(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`TrieRoute("/r1")`, l1), IsNil)
	c.Assert(r.AddLocation(`TrieRoute("/r1")`, l1), NotNil)

	// Make sure that error did not have side effects
	out, err := r.Route(makeReq("http://google.com/r1"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l1)
}

func (s *RouteSuite) TestBadExpression(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`TrieRoute("/r1")`, l1), IsNil)
	c.Assert(r.AddLocation(`Path(blabla`, l1), NotNil)

	// Make sure that error did not have side effects
	out, err := r.Route(makeReq("http://google.com/r1"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l1)
}

func (s *RouteSuite) TestTrieLegacyOperations(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`TrieRoute("/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`TrieRoute("/r2")`, l2), IsNil)

	out1, err := r.Route(makeReq("http://google.com/r1"))
	c.Assert(err, IsNil)
	c.Assert(out1, Equals, l1)

	out2, err := r.Route(makeReq("http://google.com/r2"))
	c.Assert(err, IsNil)
	c.Assert(out2, Equals, l2)
}

func (s *RouteSuite) TestTrieNewOperations(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`Path("/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`Path("/r2")`, l2), IsNil)

	out1, err := r.Route(makeReq("http://google.com/r1"))
	c.Assert(err, IsNil)
	c.Assert(out1, Equals, l1)

	out2, err := r.Route(makeReq("http://google.com/r2"))
	c.Assert(err, IsNil)
	c.Assert(out2, Equals, l2)
}

func (s *RouteSuite) TestTrieMiss(c *C) {
	r := NewExpRouter()

	c.Assert(r.AddLocation(`TrieRoute("/r1")`, makeLoc("loc1")), IsNil)

	out, err := r.Route(makeReq("http://google.com/r2"))
	c.Assert(err, IsNil)
	c.Assert(out, IsNil)
}

func (s *RouteSuite) TestRegexpOperations(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`PathRegexp("/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`PathRegexp("/r2")`, l2), IsNil)

	out, err := r.Route(makeReq("http://google.com/r1"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l1)

	out, err = r.Route(makeReq("http://google.com/r2"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l2)

	out, err = r.Route(makeReq("http://google.com/r3"))
	c.Assert(err, IsNil)
	c.Assert(out, IsNil)
}

func (s *RouteSuite) TestRegexpLegacyOperations(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`RegexpRoute("/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`RegexpRoute("/r2")`, l2), IsNil)

	out, err := r.Route(makeReq("http://google.com/r1"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l1)

	out, err = r.Route(makeReq("http://google.com/r2"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l2)

	out, err = r.Route(makeReq("http://google.com/r3"))
	c.Assert(err, IsNil)
	c.Assert(out, IsNil)
}

func (s *RouteSuite) TestMixedOperations(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`PathRegexp("/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`Path("/r2")`, l2), IsNil)

	out, err := r.Route(makeReq("http://google.com/r1"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l1)

	out, err = r.Route(makeReq("http://google.com/r2"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l2)

	out, err = r.Route(makeReq("http://google.com/r3"))
	c.Assert(err, IsNil)
	c.Assert(out, IsNil)
}

func (s *RouteSuite) TestMatchByMethodLegacy(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`TrieRoute("POST", "/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`TrieRoute("GET", "/r1")`, l2), IsNil)

	req := makeReq("http://google.com/r1")
	req.GetHttpRequest().Method = "POST"

	out, err := r.Route(req)
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l1)

	req.GetHttpRequest().Method = "GET"
	out, err = r.Route(req)
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l2)
}

func (s *RouteSuite) TestMatchByMethod(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`Method("POST") && Path("/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`Method("GET") && Path("/r1")`, l2), IsNil)

	req := makeReq("http://google.com/r1")
	req.GetHttpRequest().Method = "POST"

	out, err := r.Route(req)
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l1)

	req.GetHttpRequest().Method = "GET"
	out, err = r.Route(req)
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l2)
}

func (s *RouteSuite) TestTrieMatchLongestPath(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`Method("POST") && Path("/r")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`Method("POST") && Path("/r/hello")`, l2), IsNil)

	req := makeReq("http://google.com/r/hello")
	req.GetHttpRequest().Method = "POST"

	out, err := r.Route(req)
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l2)
}

func (s *RouteSuite) TestRegexpMatchLongestPath(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`PathRegexp("/r")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`PathRegexp("/r/hello")`, l2), IsNil)

	req := makeReq("http://google.com/r/hello")

	out, err := r.Route(req)
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l2)
}

func makeReq(url string) request.Request {
	u := netutils.MustParseUrl(url)
	return &request.BaseRequest{
		HttpRequest: &http.Request{URL: u, RequestURI: url},
	}
}

func makeLoc(url string) location.Location {
	return &location.ConstHttpLocation{Url: url}
}
