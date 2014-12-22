package limit

import (
	. "gopkg.in/check.v1"
	"testing"
)

func TestLimit(t *testing.T) { TestingT(t) }

type LimitSuite struct {
}

var _ = Suite(&LimitSuite{})

func (s *LimitSuite) TestVariableToMapper(c *C) {
	m, err := VariableToMapper("client.ip")
	c.Assert(err, IsNil)
	c.Assert(m, NotNil)

	m, err = VariableToMapper("request.host")
	c.Assert(err, IsNil)
	c.Assert(m, NotNil)

	m, err = VariableToMapper("request.header.X-Header-Name")
	c.Assert(err, IsNil)
	c.Assert(m, NotNil)

	m, err = VariableToMapper("rsom")
	c.Assert(err, NotNil)
	c.Assert(m, IsNil)
}
