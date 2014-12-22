package request

import (
	. "gopkg.in/check.v1"
	"net/http"
	"testing"
)

func TestRequest(t *testing.T) { TestingT(t) }

type RequestSuite struct {
}

var _ = Suite(&RequestSuite{})

func (s *RequestSuite) SetUpSuite(c *C) {
}

func (s *RequestSuite) TestUserDataInt(c *C) {
	br := NewBaseRequest(&http.Request{}, 0, nil)
	br.SetUserData("caller1", 100)
	data, present := br.GetUserData("caller1")

	c.Assert(present, Equals, true)
	c.Assert(data.(int), Equals, 100)

	br.SetUserData("caller2", 200)
	data, present = br.GetUserData("caller1")
	c.Assert(present, Equals, true)
	c.Assert(data.(int), Equals, 100)

	data, present = br.GetUserData("caller2")
	c.Assert(present, Equals, true)
	c.Assert(data.(int), Equals, 200)

	br.DeleteUserData("caller2")
	_, present = br.GetUserData("caller2")
	c.Assert(present, Equals, false)
}

func (s *RequestSuite) TestUserDataNil(c *C) {
	br := NewBaseRequest(&http.Request{}, 0, nil)
	_, present := br.GetUserData("caller1")
	c.Assert(present, Equals, false)
}
