package exproute

import (
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

type RouteSuite struct {
}

var _ = Suite(&RouteSuite{})

func (s *RouteSuite) TestEmptyOperationsSucceed(c *C) {
	r := NewExpRouter()

	c.Assert(r.GetLocationByExpression("bla"), IsNil)
	c.Assert(r.RemoveLocationByExpression("bla"), IsNil)
	c.Assert(r.RemoveLocationById("bla"), IsNil)
	c.Assert(r.GetLocationById("1"), IsNil)

	l, err := r.Route(makeReq("http://google.com/blabla"))
	c.Assert(err, IsNil)
	c.Assert(l, IsNil)
}

func (s *RouteSuite) TestCRUD(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`TrieRoute("/r1")`, l1), IsNil)

	c.Assert(r.GetLocationById("loc1"), Equals, l1)
	c.Assert(r.GetLocationByExpression(`TrieRoute("/r1")`), Equals, l1)

	c.Assert(r.RemoveLocationById("loc1"), IsNil)

	c.Assert(r.GetLocationById("loc1"), IsNil)
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
	c.Assert(r.AddLocation(`blabla`, l1), NotNil)

	// Make sure that error did not have side effects
	out, err := r.Route(makeReq("http://google.com/r1"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l1)
}

func (s *RouteSuite) TestTrieOperations(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`TrieRoute("/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`TrieRoute("/r2")`, l2), IsNil)

	// Make sure that compression worked and we have just one matcher
	c.Assert(len(r.matchers), Equals, 1)

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
	c.Assert(r.AddLocation(`RegexpRoute("/r1")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`TrieRoute("/r2")`, l2), IsNil)

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

func (s *RouteSuite) TestMatchByMethod(c *C) {
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

func (s *RouteSuite) TestMatchLongestPath(c *C) {
	r := NewExpRouter()

	l1 := makeLoc("loc1")
	c.Assert(r.AddLocation(`TrieRoute("POST", "/r")`, l1), IsNil)

	l2 := makeLoc("loc2")
	c.Assert(r.AddLocation(`TrieRoute("POST", "/r/hello")`, l2), IsNil)

	req := makeReq("http://google.com/r/hello")
	req.GetHttpRequest().Method = "POST"

	out, err := r.Route(req)
	c.Assert(err, IsNil)
	c.Assert(out, Equals, l2)
}
