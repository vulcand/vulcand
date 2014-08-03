package tokenbucket

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"net/http"
	"time"
)

type LimiterSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&LimiterSuite{})

func (s *LimiterSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

// We've hit the limit and were able to proceed on the next time run
func (s *LimiterSuite) TestHitLimit(c *C) {
	l, err := NewTokenLimiterWithOptions(
		MapClientIp, Rate{Units: 1, Period: time.Second}, Options{TimeProvider: s.tm})

	c.Assert(err, IsNil)
	re, err := l.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	// Next request from the same ip hits rate limit
	re, err = l.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)

	// Second later, the request from this ip will succeed
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	re, err = l.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)
}

// We've failed to extract client ip
func (s *LimiterSuite) TestFailure(c *C) {
	l, err := NewTokenLimiterWithOptions(
		MapClientIp, Rate{Units: 1, Period: time.Second}, Options{TimeProvider: s.tm})
	c.Assert(err, IsNil)

	_, err = l.ProcessRequest(makeRequest(""))
	c.Assert(err, NotNil)
}

// We've failed to extract client ip
func (s *LimiterSuite) TestInvalidParams(c *C) {
	_, err := NewTokenLimiter(nil, Rate{})
	c.Assert(err, NotNil)
}

// Make sure rates from different ips are controlled separatedly
func (s *LimiterSuite) TestIsolation(c *C) {
	l, err := NewTokenLimiterWithOptions(
		MapClientIp, Rate{Units: 1, Period: time.Second}, Options{TimeProvider: s.tm})

	re, err := l.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)
	c.Assert(re, IsNil)

	// Next request from the same ip hits rate limit
	re, err = l.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)

	// The request from other ip can proceed
	re, err = l.ProcessRequest(makeRequest("1.2.3.5"))
	c.Assert(err, IsNil)
	c.Assert(err, IsNil)
}

// Make sure that expiration works (Expiration is triggered after significant amount of time passes)
func (s *LimiterSuite) TestExpiration(c *C) {
	l, err := NewTokenLimiterWithOptions(
		MapClientIp, Rate{Units: 1, Period: time.Second}, Options{TimeProvider: s.tm})

	re, err := l.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	// Next request from the same ip hits rate limit
	re, err = l.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)

	// 24 hours later, the request from this ip will succeed
	s.tm.CurrentTime = s.tm.CurrentTime.Add(24 * time.Hour)
	re, err = l.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)
	c.Assert(re, IsNil)
}

func makeRequest(ip string) request.Request {
	return &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: ip,
		},
	}
}
