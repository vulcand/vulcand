package circuitbreaker

import (
	"time"

	"github.com/mailgun/vulcan/request"
	. "gopkg.in/check.v1"
)

type PredicatesSuite struct {
}

var _ = Suite(&PredicatesSuite{})

func (s *PredicatesSuite) TestTriggered(c *C) {
	predicates := []struct {
		Expression string
		Request    request.Request
		V          bool
	}{
		{
			Expression: "NetworkErrorRatio() > 0.5",
			Request:    makeRequest(O{stats: statsNetErrors(0.6)}),
			V:          true,
		},
		{
			Expression: "NetworkErrorRatio() < 0.5",
			Request:    makeRequest(O{stats: statsNetErrors(0.6)}),
			V:          false,
		},
		{
			Expression: "LatencyAtQuantileMS(50.0) > 50",
			Request:    makeRequest(O{stats: statsLatencyAtQuantile(50, time.Millisecond*51)}),
			V:          true,
		},
		{
			Expression: "LatencyAtQuantileMS(50.0) < 50",
			Request:    makeRequest(O{stats: statsLatencyAtQuantile(50, time.Millisecond*51)}),
			V:          false,
		},
		{
			Expression: "ResponseCodeRatio(500, 600, 0, 600) > 0.5",
			Request:    makeRequest(O{stats: statsResponseCodes(statusCode{Code: 200, Count: 5}, statusCode{Code: 500, Count: 6})}),
			V:          true,
		},
		{
			Expression: "ResponseCodeRatio(500, 600, 0, 600) > 0.5",
			Request:    makeRequest(O{stats: statsResponseCodes(statusCode{Code: 200, Count: 5}, statusCode{Code: 500, Count: 4})}),
			V:          false,
		},
	}
	for _, t := range predicates {
		p, err := ParseExpression(t.Expression)
		c.Assert(err, IsNil)
		c.Assert(p, NotNil)

		c.Assert(p(t.Request), Equals, t.V)
	}
}

func (s *PredicatesSuite) TestErrors(c *C) {
	predicates := []struct {
		Expression string
		Request    request.Request
	}{
		{
			Expression: "LatencyAtQuantileMS(40.0) > 50", // quantile not defined
			Request:    makeRequest(O{stats: statsNetErrors(0.6)}),
		},
		{
			Expression: "LatencyAtQuantileMS(40.0) > 50", // stats are not defined
			Request:    makeRequest(O{stats: nil}),
		},
		{
			Expression: "NetworkErrorRatio() > 20.0", // stats are not defined
			Request:    makeRequest(O{stats: nil}),
		},
		{
			Expression: "NetworkErrorRatio() > 20.0", // no last attempt
			Request:    makeRequest(O{noAttempts: true}),
		},
	}
	for _, t := range predicates {
		p, err := ParseExpression(t.Expression)
		c.Assert(err, IsNil)
		c.Assert(p, NotNil)

		c.Assert(p(t.Request), Equals, false)
	}
}
