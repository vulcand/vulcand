package tokenbucket

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

type LimiterSuite struct {
	clock *timetools.FreezedTime
}

var _ = Suite(&LimiterSuite{})

func (s *LimiterSuite) SetUpSuite(c *C) {
	s.clock = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *LimiterSuite) TestRateSetAdd(c *C) {
	rs := NewRateSet()

	// Invalid period
	err := rs.Add(0, 1, 1)
	c.Assert(err, NotNil)

	// Invalid Average
	err = rs.Add(time.Second, 0, 1)
	c.Assert(err, NotNil)

	// Invalid Burst
	err = rs.Add(time.Second, 1, 0)
	c.Assert(err, NotNil)

	err = rs.Add(time.Second, 1, 1)
	c.Assert(err, IsNil)
	c.Assert("map[1s:rate(1/1s, burst=1)]", Equals, fmt.Sprint(rs))
}

// We've hit the limit and were able to proceed on the next time run
func (s *LimiterSuite) TestHitLimit(c *C) {
	rates := NewRateSet()
	rates.Add(time.Second, 1, 1)
	tl, err := NewLimiter(rates, 0, limit.MapClientIp, nil, s.clock)
	c.Assert(err, IsNil)

	re, err := tl.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	// Next request from the same ip hits rate limit
	re, err = tl.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)

	// Second later, the request from this ip will succeed
	s.clock.Sleep(time.Second)
	re, err = tl.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)
}

// We've failed to extract client ip
func (s *LimiterSuite) TestFailure(c *C) {
	rates := NewRateSet()
	rates.Add(time.Second, 1, 1)
	tl, err := NewLimiter(rates, 0, limit.MapClientIp, nil, s.clock)
	c.Assert(err, IsNil)

	_, err = tl.ProcessRequest(makeRequest(""))
	c.Assert(err, NotNil)
}

func (s *LimiterSuite) TestInvalidParams(c *C) {
	// Rates are missing
	_, err := NewLimiter(nil, 0, limit.MapClientIp, nil, s.clock)
	c.Assert(err, NotNil)

	// Rates are empty
	_, err = NewLimiter(NewRateSet(), 0, limit.MapClientIp, nil, s.clock)
	c.Assert(err, NotNil)

	// Mapper is not provided
	rates := NewRateSet()
	rates.Add(time.Second, 1, 1)
	_, err = NewLimiter(rates, 0, nil, nil, s.clock)
	c.Assert(err, NotNil)

	// Mapper is not provided
	tl, err := NewLimiter(rates, 0, limit.MapClientIp, nil, s.clock)
	c.Assert(tl, NotNil)
	c.Assert(err, IsNil)
}

// Make sure rates from different ips are controlled separatedly
func (s *LimiterSuite) TestIsolation(c *C) {
	rates := NewRateSet()
	rates.Add(time.Second, 1, 1)
	tl, err := NewLimiter(rates, 0, limit.MapClientIp, nil, s.clock)

	re, err := tl.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)
	c.Assert(re, IsNil)

	// Next request from the same ip hits rate limit
	re, err = tl.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)

	// The request from other ip can proceed
	re, err = tl.ProcessRequest(makeRequest("1.2.3.5"))
	c.Assert(err, IsNil)
	c.Assert(err, IsNil)
}

// Make sure that expiration works (Expiration is triggered after significant amount of time passes)
func (s *LimiterSuite) TestExpiration(c *C) {
	rates := NewRateSet()
	rates.Add(time.Second, 1, 1)
	tl, err := NewLimiter(rates, 0, limit.MapClientIp, nil, s.clock)

	re, err := tl.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	// Next request from the same ip hits rate limit
	re, err = tl.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)

	// 24 hours later, the request from this ip will succeed
	s.clock.Sleep(24 * time.Hour)
	re, err = tl.ProcessRequest(makeRequest("1.2.3.4"))
	c.Assert(err, IsNil)
	c.Assert(re, IsNil)
}

// If configMapper returns error, then the default rate is applied.
func (s *LimiterSuite) TestBadConfigMapper(c *C) {
	// Given
	configMapper := func(r request.Request) (*RateSet, error) {
		return nil, fmt.Errorf("Boom!")
	}
	rates := NewRateSet()
	rates.Add(time.Second, 1, 1)
	tl, _ := NewLimiter(rates, 0, limit.MapClientIp, configMapper, s.clock)
	req := makeRequest("1.2.3.4")
	// When/Then: The default rate is applied, which 1 req/second
	response, err := tl.ProcessRequest(req) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = tl.ProcessRequest(req) // Rejected
	c.Assert(response, NotNil)
	c.Assert(err, IsNil)

	s.clock.Sleep(time.Second)
	response, err = tl.ProcessRequest(req) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
}

// If configMapper returns empty rates, then the default rate is applied.
func (s *LimiterSuite) TestEmptyConfig(c *C) {
	// Given
	configMapper := func(r request.Request) (*RateSet, error) {
		return NewRateSet(), nil
	}
	rates := NewRateSet()
	rates.Add(time.Second, 1, 1)
	tl, _ := NewLimiter(rates, 0, limit.MapClientIp, configMapper, s.clock)
	req := makeRequest("1.2.3.4")
	// When/Then: The default rate is applied, which 1 req/second
	response, err := tl.ProcessRequest(req) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = tl.ProcessRequest(req) // Rejected
	c.Assert(response, NotNil)
	c.Assert(err, IsNil)

	s.clock.Sleep(time.Second)
	response, err = tl.ProcessRequest(req) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
}

// If rate limiting configuration is valid, then it is applied.
func (s *LimiterSuite) TestConfigApplied(c *C) {
	// Given
	configMapper := func(request.Request) (*RateSet, error) {
		rates := NewRateSet()
		rates.Add(time.Second, 2, 2)
		rates.Add(60*time.Second, 10, 10)
		return rates, nil
	}
	rates := NewRateSet()
	rates.Add(time.Second, 1, 1)
	tl, _ := NewLimiter(rates, 0, limit.MapClientIp, configMapper, s.clock)
	req := makeRequest("1.2.3.4")
	// When/Then: The configured rate is applied, which 2 req/second
	response, err := tl.ProcessRequest(req) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = tl.ProcessRequest(req) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = tl.ProcessRequest(req) // Rejected
	c.Assert(response, NotNil)
	c.Assert(err, IsNil)

	s.clock.Sleep(time.Second)
	response, err = tl.ProcessRequest(req) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
}

func makeRequest(ip string) request.Request {
	return &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: ip,
		},
	}
}
