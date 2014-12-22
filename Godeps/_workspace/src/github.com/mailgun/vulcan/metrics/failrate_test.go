package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/mailgun/timetools"
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/request"

	. "gopkg.in/check.v1"
)

func TestFailrate(t *testing.T) { TestingT(t) }

type FailRateSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&FailRateSuite{})

func (s *FailRateSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *FailRateSuite) TestInvalidParams(c *C) {
	e := MustParseUrl("http://localhost:5000")

	// Invalid endpoint
	_, err := NewRollingMeter(nil, 10, time.Second, s.tm, nil)
	c.Assert(err, Not(IsNil))

	// Bad buckets count
	_, err = NewRollingMeter(e, 0, time.Second, s.tm, nil)
	c.Assert(err, Not(IsNil))

	// Too precise resolution
	_, err = NewRollingMeter(e, 10, time.Millisecond, s.tm, nil)
	c.Assert(err, Not(IsNil))
}

func (s *FailRateSuite) TestNotReady(c *C) {
	e := MustParseUrl("http://localhost:5000")

	// No data
	fr, err := NewRollingMeter(e, 10, time.Second, s.tm, nil)
	c.Assert(err, IsNil)
	c.Assert(fr.IsReady(), Equals, false)
	c.Assert(fr.GetRate(), Equals, 0.0)

	// Not enough data
	fr, err = NewRollingMeter(e, 10, time.Second, s.tm, nil)
	c.Assert(err, IsNil)
	fr.ObserveResponse(makeFailRequest(e))
	c.Assert(fr.IsReady(), Equals, false)
}

// Make sure we don't count the stats from the endpoints we don't care or requests with no attempts
func (s *FailRateSuite) TestIgnoreOtherEndpoints(c *C) {
	e := MustParseUrl("http://localhost:5000")
	e2 := MustParseUrl("http://localhost:5001")

	fr, err := NewRollingMeter(e, 1, time.Second, s.tm, nil)
	c.Assert(err, IsNil)
	fr.ObserveResponse(makeFailRequest(e))
	fr.ObserveResponse(makeFailRequest(e2))

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, 1.0)
}

func (s *FailRateSuite) TestIgnoreRequestsWithoutAttempts(c *C) {
	e := MustParseUrl("http://localhost:5000")

	fr, err := NewRollingMeter(e, 1, time.Second, s.tm, nil)
	c.Assert(err, IsNil)
	fr.ObserveResponse(makeFailRequest(e))
	fr.ObserveResponse(&BaseRequest{}, nil)

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, 1.0)
}

func (s *FailRateSuite) TestNoSuccesses(c *C) {
	e := MustParseUrl("http://localhost:5000")

	fr, err := NewRollingMeter(e, 1, time.Second, s.tm, nil)
	c.Assert(err, IsNil)
	fr.ObserveResponse(makeFailRequest(e))

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, 1.0)
}

func (s *FailRateSuite) TestNoFailures(c *C) {
	e := MustParseUrl("http://localhost:5000")

	fr, err := NewRollingMeter(e, 1, time.Second, s.tm, nil)
	c.Assert(err, IsNil)
	fr.ObserveResponse(makeOkRequest(e))

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, 0.0)
}

// Make sure that data is properly calculated over several buckets
func (s *FailRateSuite) TestMultipleBuckets(c *C) {
	e := MustParseUrl("http://localhost:5000")

	fr, err := NewRollingMeter(e, 3, time.Second, s.tm, nil)
	c.Assert(err, IsNil)

	fr.ObserveResponse(makeOkRequest(e))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, float64(2)/float64(3))
}

// Make sure that data is properly calculated over several buckets
// When we overwrite old data when the window is rolling
func (s *FailRateSuite) TestOverwriteBuckets(c *C) {
	e := MustParseUrl("http://localhost:5000")

	fr, err := NewRollingMeter(e, 3, time.Second, s.tm, nil)
	c.Assert(err, IsNil)

	fr.ObserveResponse(makeOkRequest(e))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))

	// This time we should overwrite the old data points
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))
	fr.ObserveResponse(makeOkRequest(e))
	fr.ObserveResponse(makeOkRequest(e))

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, float64(3)/float64(5))
}

// Make sure we cleanup the data after periods of inactivity
// So it does not mess up the stats
func (s *FailRateSuite) TestInactiveBuckets(c *C) {
	e := MustParseUrl("http://localhost:5000")

	fr, err := NewRollingMeter(e, 3, time.Second, s.tm, nil)
	c.Assert(err, IsNil)

	fr.ObserveResponse(makeOkRequest(e))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))

	// This time we should overwrite the old data points with new data
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))
	fr.ObserveResponse(makeOkRequest(e))
	fr.ObserveResponse(makeOkRequest(e))

	// Jump to the last bucket and change the data
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second * 2)
	fr.ObserveResponse(makeOkRequest(e))

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, float64(1)/float64(4))
}

func (s *FailRateSuite) TestLongPeriodsOfInactivity(c *C) {
	e := MustParseUrl("http://localhost:5000")

	fr, err := NewRollingMeter(e, 2, time.Second, s.tm, nil)
	c.Assert(err, IsNil)

	fr.ObserveResponse(makeOkRequest(e))

	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	fr.ObserveResponse(makeFailRequest(e))

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, 0.5)

	// This time we should overwrite all data points
	s.tm.CurrentTime = s.tm.CurrentTime.Add(100 * time.Second)
	fr.ObserveResponse(makeFailRequest(e))
	c.Assert(fr.GetRate(), Equals, 1.0)
}

func (s *FailRateSuite) TestReset(c *C) {
	e := MustParseUrl("http://localhost:5000")

	fr, err := NewRollingMeter(e, 1, time.Second, s.tm, nil)
	c.Assert(err, IsNil)

	fr.ObserveResponse(makeOkRequest(e))
	fr.ObserveResponse(makeFailRequest(e))

	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, 0.5)

	// Reset the counter
	fr.Reset()
	c.Assert(fr.IsReady(), Equals, false)

	// Now add some stats
	fr.ObserveResponse(makeFailRequest(e))
	fr.ObserveResponse(makeFailRequest(e))

	// We are game again!
	c.Assert(fr.IsReady(), Equals, true)
	c.Assert(fr.GetRate(), Equals, 1.0)
}

func makeRequest(endpoint Endpoint, err error) Request {
	return &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Error:    err,
				Endpoint: endpoint,
			},
		},
	}
}

func makeFailRequest(endpoint Endpoint) (Request, Attempt) {
	r := makeRequest(endpoint, fmt.Errorf("Oops"))
	return r, r.GetLastAttempt()
}

func makeOkRequest(endpoint Endpoint) (Request, Attempt) {
	r := makeRequest(endpoint, nil)
	return r, r.GetLastAttempt()
}
