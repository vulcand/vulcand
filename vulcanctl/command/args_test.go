package command

import (
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/launchpad.net/gocheck"
)

type ArgsSuite struct {
}

var _ = Suite(&ArgsSuite{})

func (s *ArgsSuite) TestFindVulcanUrl(c *C) {
	url, args, err := findVulcanUrl([]string{"vulcanctl", "--vulcan=bla"})
	c.Assert(err, IsNil)
	c.Assert(url, Equals, "bla")
	c.Assert(args, DeepEquals, []string{"vulcanctl"})
}

func (s *ArgsSuite) TestFindDefaults(c *C) {
	url, args, err := findVulcanUrl([]string{"vulcanctl", "status"})
	c.Assert(err, IsNil)
	c.Assert(url, Equals, "http://localhost:8182")
	c.Assert(args, DeepEquals, []string{"vulcanctl", "status"})
}

func (s *ArgsSuite) TestFindMiddle(c *C) {
	url, args, err := findVulcanUrl([]string{"vulcanctl", "endpoint", "-vulcan", "http://yo", "rm"})
	c.Assert(err, IsNil)
	c.Assert(url, Equals, "http://yo")
	c.Assert(args, DeepEquals, []string{"vulcanctl", "endpoint", "rm"})
}

func (s *ArgsSuite) TestFindNoUrl(c *C) {
	_, _, err := findVulcanUrl([]string{"vulcanctl", "endpoint", "rm", "-vulcan"})
	c.Assert(err, NotNil)
}
