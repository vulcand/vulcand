package backend

import (
	. "launchpad.net/gocheck"
	"testing"
)

func TestBackend(t *testing.T) { TestingT(t) }

type BackendSuite struct {
}

var _ = Suite(&BackendSuite{})

func (s *BackendSuite) TestVariableToMapper(c *C) {
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

func (s *BackendSuite) TestNewRateLimitSuccess(c *C) {
	rl, err := NewRateLimit(10, "client.ip", 10, 1)
	c.Assert(err, IsNil)
	c.Assert(rl, NotNil)
}

func (s *BackendSuite) TestNewRateLimitBadParams(c *C) {
	params := []struct {
		Requests      int
		Variable      string
		Burst         int
		PeriodSeconds int
	}{
		{10, "clientip", 10, 1},
		{-1, "client.ip", 10, 1},
		{10, "client.ip", -1, 1},
		{10, "client.ip", 10, -1},
	}
	for _, p := range params {
		rl, err := NewRateLimit(p.Requests, p.Variable, p.Burst, p.PeriodSeconds)
		c.Assert(err, NotNil)
		c.Assert(rl, IsNil)
	}
}

func (s *BackendSuite) TestNewConnLimitSuccess(c *C) {
	cl, err := NewConnLimit(10, "client.ip")
	c.Assert(err, IsNil)
	c.Assert(cl, NotNil)
}

func (s *BackendSuite) TestNewConnLimitBadParams(c *C) {
	params := []struct {
		Requests int
		Variable string
	}{
		{10, "clientip"},
		{-1, "client.ip"},
	}
	for _, p := range params {
		rl, err := NewConnLimit(p.Requests, p.Variable)
		c.Assert(err, NotNil)
		c.Assert(rl, IsNil)
	}
}
