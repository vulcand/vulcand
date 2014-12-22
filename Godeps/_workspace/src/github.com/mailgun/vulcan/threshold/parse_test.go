package threshold

import (
	"fmt"
	"net/http"
	"testing"

	. "github.com/mailgun/vulcan/request"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type ThresholdSuite struct {
}

var _ = Suite(&ThresholdSuite{})

func (s *ThresholdSuite) TestSuccessOnGets(c *C) {
	p, err := ParseExpression(`RequestMethod() == "GET"`)
	c.Assert(err, IsNil)

	c.Assert(p(&BaseRequest{HttpRequest: &http.Request{Method: "GET"}}), Equals, true)
	c.Assert(p(&BaseRequest{HttpRequest: &http.Request{Method: "POST"}}), Equals, false)
}

func (s *ThresholdSuite) TestSuccessOnGetsLegacy(c *C) {
	p, err := ParseExpression(`RequestMethodEq("GET")`)
	c.Assert(err, IsNil)

	c.Assert(p(&BaseRequest{HttpRequest: &http.Request{Method: "GET"}}), Equals, true)
	c.Assert(p(&BaseRequest{HttpRequest: &http.Request{Method: "POST"}}), Equals, false)
}

func (s *ThresholdSuite) TestSuccessOnGetsAndErrors(c *C) {
	p, err := ParseExpression(`(RequestMethod() == "GET") && IsNetworkError()`)
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

func (s *ThresholdSuite) TestLegacyIsNetworkError(c *C) {
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

func (s *ThresholdSuite) TestResponseCodeOrError(c *C) {
	p, err := ParseExpression(`ResponseCode() == 503 || IsNetworkError()`)
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

func (s *ThresholdSuite) TestAttemptsLeLegacy(c *C) {
	p, err := ParseExpression(`AttemptsLe(1)`)
	c.Assert(err, IsNil)

	req := &BaseRequest{
		Attempts: []Attempt{},
	}
	c.Assert(p(req), Equals, true)

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

func (s *ThresholdSuite) TestAttemptsLT(c *C) {
	p, err := ParseExpression(`Attempts() < 1`)
	c.Assert(err, IsNil)

	req := &BaseRequest{
		Attempts: []Attempt{},
	}
	c.Assert(p(req), Equals, true)

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

func (s *ThresholdSuite) TestAttemptsGT(c *C) {
	p, err := ParseExpression(`Attempts() > 1`)
	c.Assert(err, IsNil)

	req := &BaseRequest{
		Attempts: []Attempt{},
	}
	c.Assert(p(req), Equals, false)

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
	c.Assert(p(req), Equals, true)
}

func (s *ThresholdSuite) TestAttemptsGE(c *C) {
	p, err := ParseExpression(`Attempts() >= 1`)
	c.Assert(err, IsNil)

	req := &BaseRequest{
		Attempts: []Attempt{},
	}
	c.Assert(p(req), Equals, false)

	req = &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Response: &http.Response{StatusCode: 503},
			},
		},
	}
	c.Assert(p(req), Equals, true)

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
	c.Assert(p(req), Equals, true)
}

func (s *ThresholdSuite) TestAttemptsNE(c *C) {
	p, err := ParseExpression(`Attempts() != 1`)
	c.Assert(err, IsNil)

	req := &BaseRequest{
		Attempts: []Attempt{},
	}
	c.Assert(p(req), Equals, true)

	req = &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Response: &http.Response{StatusCode: 503},
			},
		},
	}
	c.Assert(p(req), Equals, false)
}

func (s *ThresholdSuite) TestComplexExpression(c *C) {
	p, err := ParseExpression(`(ResponseCode() == 503 || IsNetworkError()) && Attempts() <= 1`)
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

func (s *ThresholdSuite) TestComplexLegacyExpression(c *C) {
	p, err := ParseExpression(`(IsNetworkError || ResponseCodeEq(503)) && AttemptsLe(2)`)
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

	// Network error and one attempt
	req = &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Error: fmt.Errorf("Something failed"),
			},
		},
	}
	c.Assert(p(req), Equals, true)

	// 503 error and three attempts
	req = &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Response: &http.Response{StatusCode: 503},
			},
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

func (s *ThresholdSuite) TestInvalidCases(c *C) {
	cases := []string{
		")(",                                 // invalid expression
		"1",                                  // standalone literal
		"SomeFunc",                           // unsupported id
		"RequestMethod() == banana",          // unsupported argument
		"RequestMethod() == RequestMethod()", // unsupported argument
		"RequestMethod() == 0.2",             // unsupported argument
		"RequestMethod(200) ==  200",         // wrong number of arguments
		`RequestMethod() == "POST" && 1`,     // standalone literal in expression
		`1 && RequestMethod() == "POST"`,     // standalone literal in expression
		`Req(1)`,           // unknown method call
		`RequestMethod(1)`, // bad parameter type
	}
	for _, tc := range cases {
		p, err := ParseExpression(tc)
		c.Assert(err, NotNil)
		c.Assert(p, IsNil)
	}
}
