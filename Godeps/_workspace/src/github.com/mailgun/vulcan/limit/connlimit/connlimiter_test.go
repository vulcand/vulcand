package connlimit

import (
	"net/http"
	"testing"

	"github.com/mailgun/vulcan/request"
	. "gopkg.in/check.v1"
)

func TestConn(t *testing.T) { TestingT(t) }

type ConnLimiterSuite struct {
}

var _ = Suite(&ConnLimiterSuite{})

func (s *ConnLimiterSuite) SetUpSuite(c *C) {
}

// We've hit the limit and were able to proceed once the request has completed
func (s *ConnLimiterSuite) TestHitLimitAndRelease(c *C) {
	l, err := NewClientIpLimiter(1)
	c.Assert(err, Equals, nil)

	r := makeRequest("1.2.3.4")

	re, err := l.ProcessRequest(r)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	// Next request from the same ip hits rate limit, because the active connections > 1
	re, err = l.ProcessRequest(r)
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)

	// Once the first request finished, next one succeeds
	l.ProcessResponse(r, nil)

	re, err = l.ProcessRequest(r)
	c.Assert(err, IsNil)
	c.Assert(re, IsNil)
}

// Make sure connections are counted independently for different ips
func (s *ConnLimiterSuite) TestDifferentIps(c *C) {
	l, err := NewClientIpLimiter(1)
	c.Assert(err, Equals, nil)

	r := makeRequest("1.2.3.4")
	r2 := makeRequest("1.2.3.5")

	re, err := l.ProcessRequest(r)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	re, err = l.ProcessRequest(r)
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)

	re, err = l.ProcessRequest(r2)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)
}

// Make sure connections are counted independently for different ips
func (s *ConnLimiterSuite) TestConnectionCount(c *C) {
	l, err := NewClientIpLimiter(1)
	c.Assert(err, Equals, nil)

	r := makeRequest("1.2.3.4")
	r2 := makeRequest("1.2.3.5")

	re, err := l.ProcessRequest(r)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)
	c.Assert(l.GetConnectionCount(), Equals, int64(1))

	re, err = l.ProcessRequest(r)
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)
	c.Assert(l.GetConnectionCount(), Equals, int64(1))

	re, err = l.ProcessRequest(r2)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)
	c.Assert(l.GetConnectionCount(), Equals, int64(2))

	l.ProcessResponse(r, nil)
	c.Assert(l.GetConnectionCount(), Equals, int64(1))

	l.ProcessResponse(r2, nil)
	c.Assert(l.GetConnectionCount(), Equals, int64(0))
}

// We've failed to extract client ip, everything crashes, bam!
func (s *ConnLimiterSuite) TestFailure(c *C) {
	l, err := NewClientIpLimiter(1)
	c.Assert(err, IsNil)
	re, err := l.ProcessRequest(makeRequest(""))
	c.Assert(err, NotNil)
	c.Assert(re, IsNil)
}

func (s *ConnLimiterSuite) TestWrongParams(c *C) {
	_, err := NewConnectionLimiter(nil, 1)
	c.Assert(err, NotNil)

	_, err = NewClientIpLimiter(0)
	c.Assert(err, NotNil)

	_, err = NewClientIpLimiter(-1)
	c.Assert(err, NotNil)
}

func makeRequest(ip string) request.Request {
	return &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: ip,
		},
	}
}
