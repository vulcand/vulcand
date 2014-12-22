package pathroute

import (
	"fmt"
	. "github.com/mailgun/vulcan/location"
	. "github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcan/testutils"
	. "gopkg.in/check.v1"
	"net/http"
	"testing"
)

func TestPathRoute(t *testing.T) { TestingT(t) }

type MatchSuite struct {
}

var _ = Suite(&MatchSuite{})

func (s *MatchSuite) SetUpSuite(c *C) {
}

func (s *MatchSuite) TestRouteEmpty(c *C) {
	m := NewPathRouter()
	out, err := m.Route(request("http://google.com/"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, nil)
}

func (s *MatchSuite) TestRemoveNonExistent(c *C) {
	m := NewPathRouter()
	c.Assert(m.RemoveLocation(m.GetLocationByPattern("ooo")), Not(Equals), nil)
}

func (s *MatchSuite) TestAddTwice(c *C) {
	m := NewPathRouter()
	loc := &Loc{Name: "a"}
	c.Assert(m.AddLocation("/a", loc), IsNil)
	c.Assert(m.AddLocation("/a", loc), Not(Equals), nil)
}

func (s *MatchSuite) TestSingleLocation(c *C) {
	m := NewPathRouter()
	loc := &Loc{Name: "a"}
	c.Assert(m.AddLocation("/", loc), IsNil)
	out, err := m.Route(request("http://google.com/"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, loc)
}

func (s *MatchSuite) TestEmptyPath(c *C) {
	m := NewPathRouter()
	loc := &Loc{Name: "a"}
	c.Assert(m.AddLocation("/", loc), IsNil)
	out, err := m.Route(request("http://google.com"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, loc)
}

func (s *MatchSuite) TestMatchNothing(c *C) {
	m := NewPathRouter()
	loc := &Loc{Name: "a"}
	c.Assert(m.AddLocation("/", loc), IsNil)
	out, err := m.Route(request("http://google.com/hello/there"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, nil)
}

// Make sure we'll match request regardless if it has trailing slash or not
func (s *MatchSuite) TestTrailingSlashes(c *C) {
	m := NewPathRouter()
	loc := &Loc{Name: "a"}
	c.Assert(m.AddLocation("/a/b", loc), IsNil)

	out, err := m.Route(request("http://google.com/a/b"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, loc)

	out, err = m.Route(request("http://google.com/a/b/"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, loc)
}

// If users added trailing slashes the request will require them to match request
func (s *MatchSuite) TestPatternTrailingSlashes(c *C) {
	m := NewPathRouter()
	loc := &Loc{Name: "a"}
	c.Assert(m.AddLocation("/a/b/", loc), IsNil)

	out, err := m.Route(request("http://google.com/a/b"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, nil)

	out, err = m.Route(request("http://google.com/a/b/"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, loc)
}

func (s *MatchSuite) TestMultipleLocations(c *C) {
	m := NewPathRouter()
	locA := &Loc{Name: "a"}
	locB := &Loc{Name: "b"}

	c.Assert(m.AddLocation("/a/there", locA), IsNil)
	c.Assert(m.AddLocation("/c", locB), IsNil)

	out, err := m.Route(request("http://google.com/a/there"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locA)

	out, err = m.Route(request("http://google.com/c"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locB)
}

func (s *MatchSuite) TestChooseLongest(c *C) {
	m := NewPathRouter()
	locA := &Loc{Name: "a"}
	locB := &Loc{Name: "b"}

	c.Assert(m.AddLocation("/a/there", locA), IsNil)
	c.Assert(m.AddLocation("/a", locB), IsNil)

	out, err := m.Route(request("http://google.com/a/there"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locA)

	out, err = m.Route(request("http://google.com/a"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locB)
}

func (s *MatchSuite) TestRemove(c *C) {
	m := NewPathRouter()
	locA := &Loc{Name: "a"}
	locB := &Loc{Name: "b"}

	c.Assert(m.AddLocation("/a", locA), IsNil)
	c.Assert(m.AddLocation("/b", locB), IsNil)

	out, err := m.Route(request("http://google.com/a"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locA)

	out, err = m.Route(request("http://google.com/b"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locB)

	// Remove the location and make sure the matcher is still valid
	c.Assert(m.RemoveLocation(m.GetLocationByPattern("/b")), IsNil)

	out, err = m.Route(request("http://google.com/a"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locA)

	out, err = m.Route(request("http://google.com/b"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, nil)
}

func (s *MatchSuite) TestAddBad(c *C) {
	m := NewPathRouter()
	locA := &Loc{Name: "a"}
	locB := &Loc{Name: "b"}

	c.Assert(m.AddLocation("/a/there", locA), IsNil)

	out, err := m.Route(request("http://google.com/a/there"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locA)

	c.Assert(m.AddLocation("--(", locB), Not(Equals), nil)

	out, err = m.Route(request("http://google.com/a/there"))
	c.Assert(err, IsNil)
	c.Assert(out, Equals, locA)
}

func (s *MatchSuite) BenchmarkMatching(c *C) {
	rndString := testutils.NewRndString()

	m := NewPathRouter()
	loc := &Loc{Name: "a"}

	for i := 0; i < 100; i++ {
		err := m.AddLocation(rndString.MakePath(20, 10), loc)
		c.Assert(err, IsNil)
	}

	req := request(fmt.Sprintf("http://google.com/%s", rndString.MakePath(20, 10)))
	for i := 0; i < c.N; i++ {
		m.Route(req)
	}
}

func request(url string) Request {
	u := MustParseUrl(url)
	return &BaseRequest{
		HttpRequest: &http.Request{URL: u},
	}
}
