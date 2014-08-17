package failover

import (
	"fmt"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"net/http"
	"testing"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

func Test(t *testing.T) { TestingT(t) }

type FailoverSuite struct {
}

var _ = Suite(&FailoverSuite{})

func (s *FailoverSuite) TestSuccessOnGets(c *C) {
	p, err := ParseExpression(`RequestMethodEq("GET")`)
	c.Assert(err, IsNil)

	c.Assert(p(&BaseRequest{HttpRequest: &http.Request{Method: "GET"}}), Equals, true)
	c.Assert(p(&BaseRequest{HttpRequest: &http.Request{Method: "POST"}}), Equals, false)
}

func (s *FailoverSuite) TestSuccessOnGetsAndErrors(c *C) {
	p, err := ParseExpression(`RequestMethodEq("GET") && IsNetworkError`)
	c.Assert(err, IsNil)

	// There's no error
	c.Assert(p(&BaseRequest{HttpRequest: &http.Request{Method: "GET"}}), Equals, false)

	// This one allows error
	req := &BaseRequest{
		HttpRequest: &http.Request{Method: "GET"},
		Attempts: []Attempt{
			&BaseAttempt{
				Error: fmt.Errorf("Something failed"),
			},
		},
	}
	c.Assert(p(req), Equals, true)
}

func (s *FailoverSuite) TestResponseCodeOrError(c *C) {
	p, err := ParseExpression(`ResponseCodeEq(503) || IsNetworkError`)
	c.Assert(err, IsNil)

	// There's no error
	c.Assert(p(&BaseRequest{}), Equals, false)

	// There's a network error
	req := &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Error: fmt.Errorf("Something failed"),
			},
		},
	}
	c.Assert(p(req), Equals, true)

	// There's a 503 response code
	req = &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Response: &http.Response{StatusCode: 503},
			},
		},
	}
	c.Assert(p(req), Equals, true)

	// Different response code does not work
	req = &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Response: &http.Response{StatusCode: 504},
			},
		},
	}
	c.Assert(p(req), Equals, false)
}

func (s *FailoverSuite) TestComplexExpression(c *C) {
	p, err := ParseExpression(`(ResponseCodeEq(503) || IsNetworkError) && AttemptsLe(1)`)
	c.Assert(err, IsNil)

	// 503 error and one attempt
	req := &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Response: &http.Response{StatusCode: 503},
			},
		},
	}
	c.Assert(p(req), Equals, true)

	// 503 error and more than one attempt
	req = &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Response: &http.Response{StatusCode: 503},
			},
			&BaseAttempt{
				Response: &http.Response{StatusCode: 503},
			},
		},
	}
	c.Assert(p(req), Equals, false)
}

func (s *FailoverSuite) TestInvalidCases(c *C) {
	cases := []string{
		")(",                                       // invalid expression
		"1",                                        // standalone literal
		"SomeFunc",                                 // unsupported id
		"RequestMethodEq(banana)",                  // unsupported argument
		"RequestMethodEq(MethodEq)",                // unsupported argument
		"RequestMethodEq(0.2)",                     // unsupported argument
		"RequestMethodEq(200, 200)",                // wrong number of arguments
		`RequestMethodEq("POST") && 1`,             // standalone literal in expression
		`1 && RequestMethodEq("POST")`,             // standalone literal in expression
		`RequestMethodEq("POST") | IsNetworkError`, // unsupported binary operator
		`Req(1)`,             // unknown method call
		`RequestMethodEq(1)`, // bad parameter type
	}
	for _, tc := range cases {
		p, err := ParseExpression(tc)
		c.Assert(err, NotNil)
		c.Assert(p, IsNil)
	}
}
