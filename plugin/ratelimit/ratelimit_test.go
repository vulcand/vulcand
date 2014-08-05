package ratelimit

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/plugin"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"testing"
)

func TestRL(t *testing.T) { TestingT(t) }

type RateLimitSuite struct {
}

var _ = Suite(&RateLimitSuite{})

// One of the most important tests:
// Make sure the RateLimit spec is compatible and will be accepted by middleware registry
func (s *RateLimitSuite) TestSpecIsOK(c *C) {
	c.Assert(plugin.NewRegistry().AddSpec(GetSpec()), IsNil)
}

func (s *RateLimitSuite) TestNewRateLimitSuccess(c *C) {
	rl, err := NewRateLimit(10, "client.ip", 1, 1)
	c.Assert(rl, NotNil)
	c.Assert(err, IsNil)

	c.Assert(rl.String(), Not(Equals), "")

	out, err := rl.NewMiddleware()
	c.Assert(out, NotNil)
	c.Assert(err, IsNil)
}

func (s *RateLimitSuite) TestNewRateLimitBadParams(c *C) {
	// Unknown variable
	_, err := NewRateLimit(10, "client ip", 1, 1)
	c.Assert(err, NotNil)

	// Negative requests
	_, err = NewRateLimit(-10, "client.ip", 1, 1)
	c.Assert(err, NotNil)

	// Negative burst
	_, err = NewRateLimit(10, "client.ip", -1, 1)
	c.Assert(err, NotNil)

	// Negative period
	_, err = NewRateLimit(10, "client.ip", 1, -1)
	c.Assert(err, NotNil)
}

func (s *RateLimitSuite) TestNewRateLimitFromOther(c *C) {
	rl, err := NewRateLimit(10, "client.ip", 1, 1)
	c.Assert(rl, NotNil)
	c.Assert(err, IsNil)

	out, err := FromOther(*rl)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, rl)
}

func (s *RateLimitSuite) TestNewRateLimitFromCliOk(c *C) {
	app := cli.NewApp()
	app.Name = "test"
	executed := false
	app.Action = func(ctx *cli.Context) {
		executed = true
		out, err := FromCli(ctx)
		c.Assert(out, NotNil)
		c.Assert(err, IsNil)

		rl := out.(*RateLimit)
		c.Assert(rl.Variable, Equals, "client.ip")
		c.Assert(rl.Requests, Equals, 10)
		c.Assert(rl.Burst, Equals, 3)
		c.Assert(rl.PeriodSeconds, Equals, 4)
	}
	app.Flags = CliFlags()
	app.Run([]string{"test", "--var=client.ip", "--requests=10", "--burst=3", "--period=4"})
	c.Assert(executed, Equals, true)
}
