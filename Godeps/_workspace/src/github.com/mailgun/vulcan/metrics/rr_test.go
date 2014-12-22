package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mailgun/timetools"
	"github.com/mailgun/vulcan/request"
	. "gopkg.in/check.v1"
)

type RRSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&RRSuite{})

func (s *RRSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *RRSuite) TestDefaults(c *C) {
	rr, err := NewRoundTripMetrics(RoundTripOptions{TimeProvider: s.tm})
	c.Assert(err, IsNil)
	c.Assert(rr, NotNil)

	rr.RecordMetrics(makeAttempt(O{err: fmt.Errorf("o"), duration: time.Second}))
	rr.RecordMetrics(makeAttempt(O{statusCode: 500, duration: 2 * time.Second}))
	rr.RecordMetrics(makeAttempt(O{statusCode: 200, duration: time.Second}))
	rr.RecordMetrics(makeAttempt(O{statusCode: 200, duration: time.Second}))

	c.Assert(rr.GetNetworkErrorCount(), Equals, int64(1))
	c.Assert(rr.GetTotalCount(), Equals, int64(4))
	c.Assert(rr.GetStatusCodesCounts(), DeepEquals, map[int]int64{500: 1, 200: 2})
	c.Assert(rr.GetNetworkErrorRatio(), Equals, float64(1)/float64(4))
	c.Assert(rr.GetResponseCodeRatio(500, 501, 200, 300), Equals, 0.5)

	h, err := rr.GetLatencyHistogram()
	c.Assert(err, IsNil)
	c.Assert(int(h.LatencyAtQuantile(100)/time.Second), Equals, 2)

	rr.Reset()
	c.Assert(rr.GetNetworkErrorCount(), Equals, int64(0))
	c.Assert(rr.GetTotalCount(), Equals, int64(0))
	c.Assert(rr.GetStatusCodesCounts(), DeepEquals, map[int]int64{})
	c.Assert(rr.GetNetworkErrorRatio(), Equals, float64(0))
	c.Assert(rr.GetResponseCodeRatio(500, 501, 200, 300), Equals, float64(0))

	h, err = rr.GetLatencyHistogram()
	c.Assert(err, IsNil)
	c.Assert(h.LatencyAtQuantile(100), Equals, time.Duration(0))
}

func makeAttempt(o O) *request.BaseAttempt {
	a := &request.BaseAttempt{
		Error:    o.err,
		Duration: o.duration,
	}
	if o.statusCode != 0 {
		a.Response = &http.Response{StatusCode: o.statusCode}
	}
	return a
}

type O struct {
	statusCode int
	err        error
	duration   time.Duration
}
