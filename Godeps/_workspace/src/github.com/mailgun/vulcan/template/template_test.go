package template

import (
	"net/http"
	"testing"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestTemplate(t *testing.T) { TestingT(t) }

type TemplateSuite struct{}

var _ = Suite(&TemplateSuite{})

func (s *TemplateSuite) SetUpSuite(c *C) {
}

func (s *TemplateSuite) TestTemplateOkay(c *C) {
	request, _ := http.NewRequest("GET", "http://foo", nil)
	request.Header.Add("X-Header", "bar")

	new, err := Apply(`foo {{.Request.Header.Get "X-Header"}}`, Data{request})
	c.Assert(err, IsNil)
	c.Assert(new, Equals, "foo bar")
}

func (s *TemplateSuite) TestBadTemplate(c *C) {
	request, _ := http.NewRequest("GET", "http://foo", nil)
	request.Header.Add("X-Header", "bar")

	old := `foo {{.Request.Header.Get "X-Header"`
	new, err := Apply(old, Data{request})
	c.Assert(err, NotNil)
	c.Assert(new, Equals, old)
}

func (s *TemplateSuite) TestNoVariables(c *C) {
	request, _ := http.NewRequest("GET", "http://foo", nil)
	request.Header.Add("X-Header", "bar")

	new, err := Apply(`foo baz`, Data{request})
	c.Assert(err, IsNil)
	c.Assert(new, Equals, "foo baz")
}

func (s *TemplateSuite) TestNonexistentVariable(c *C) {
	request, _ := http.NewRequest("GET", "http://foo", nil)
	request.Header.Add("X-Header", "bar")

	new, err := Apply(`foo {{.Request.Header.Get "Y-Header"}}`, Data{request})
	c.Assert(err, IsNil)
	c.Assert(new, Equals, "foo ")
}
