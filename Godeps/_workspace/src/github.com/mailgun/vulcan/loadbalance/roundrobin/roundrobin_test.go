package roundrobin

import (
	"fmt"
	"github.com/mailgun/timetools"
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/metrics"
	. "github.com/mailgun/vulcan/request"
	. "gopkg.in/check.v1"
	"testing"
	"time"
)

func Test(t *testing.T) { TestingT(t) }

type RoundRobinSuite struct {
	tm  *timetools.FreezedTime
	req Request
}

var _ = Suite(&RoundRobinSuite{})

func (s *RoundRobinSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
	s.req = &BaseRequest{}
}

func (s *RoundRobinSuite) newRR() *RoundRobin {
	handler, err := NewFSMHandlerWithOptions(s.tm)
	if err != nil {
		panic(err)
	}

	r, err := NewRoundRobinWithOptions(Options{TimeProvider: s.tm, FailureHandler: handler})
	if err != nil {
		panic(err)
	}
	return r
}

func (s *RoundRobinSuite) TestNoEndpoints(c *C) {
	r := s.newRR()
	_, err := r.NextEndpoint(s.req)
	c.Assert(err, NotNil)
}

func (s *RoundRobinSuite) TestDefaultArgs(c *C) {
	r, err := NewRoundRobin()
	c.Assert(err, IsNil)

	a := MustParseUrl("http://localhost:5000")
	b := MustParseUrl("http://localhost:5001")

	r.AddEndpoint(a)
	r.AddEndpoint(b)

	u, err := r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, a)

	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, b)

	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, a)
}

// Subsequent calls to load balancer with 1 endpoint are ok
func (s *RoundRobinSuite) TestSingleEndpoint(c *C) {
	r := s.newRR()

	u := MustParseUrl("http://localhost:5000")
	r.AddEndpoint(u)

	u2, err := r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u2, Equals, u)

	u3, err := r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u3, Equals, u)
}

// Make sure that load balancer round robins requests
func (s *RoundRobinSuite) TestMultipleEndpoints(c *C) {
	r := s.newRR()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	r.AddEndpoint(uA)
	r.AddEndpoint(uB)

	u, err := r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uB)

	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)
}

// Make sure that adding endpoints during load balancing works fine
func (s *RoundRobinSuite) TestAddEndpoints(c *C) {
	r := s.newRR()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	r.AddEndpoint(uA)

	u, err := r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	r.AddEndpoint(uB)

	// index was reset after altering endpoints
	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uB)
}

// Removing endpoints from the load balancer works fine as well
func (s *RoundRobinSuite) TestRemoveEndpoint(c *C) {
	r := s.newRR()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	r.AddEndpoint(uA)
	r.AddEndpoint(uB)

	u, err := r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)

	// Removing endpoint resets the counter
	r.RemoveEndpoint(uB)

	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uA)
}

func (s *RoundRobinSuite) TestAddSameEndpoint(c *C) {
	r := s.newRR()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5000")
	r.AddEndpoint(uA)
	c.Assert(r.AddEndpoint(uB), NotNil)
}

func (s *RoundRobinSuite) TestFindEndpoint(c *C) {
	r := s.newRR()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	r.AddEndpoint(uA)
	r.AddEndpoint(uB)

	c.Assert(r.FindEndpointById(""), IsNil)
	c.Assert(r.FindEndpointById(uA.GetId()).GetId(), Equals, uA.GetId())
	c.Assert(r.FindEndpointByUrl(uA.GetUrl().String()).GetId(), Equals, uA.GetId())
	c.Assert(r.FindEndpointByUrl(""), IsNil)
	c.Assert(r.FindEndpointByUrl("http://localhost wrong url 5000"), IsNil)
}

func (s *RoundRobinSuite) advanceTime(d time.Duration) {
	s.tm.CurrentTime = s.tm.CurrentTime.Add(d)
}

func (s *RoundRobinSuite) TestReactsOnFailures(c *C) {
	handler, err := NewFSMHandlerWithOptions(s.tm)
	c.Assert(err, IsNil)

	r, err := NewRoundRobinWithOptions(
		Options{
			TimeProvider:   s.tm,
			FailureHandler: handler,
		})
	c.Assert(err, IsNil)

	a := MustParseUrl("http://localhost:5000")
	aM := &TestMeter{Rate: 0.5}

	b := MustParseUrl("http://localhost:5001")
	bM := &TestMeter{Rate: 0}

	r.AddEndpointWithOptions(a, EndpointOptions{Meter: aM})
	r.AddEndpointWithOptions(b, EndpointOptions{Meter: bM})

	countA, countB := 0, 0
	for i := 0; i < 100; i += 1 {
		e, err := r.NextEndpoint(s.req)
		if e.GetId() == a.GetId() {
			countA += 1
		} else {
			countB += 1
		}
		c.Assert(e, NotNil)
		c.Assert(err, IsNil)
		s.advanceTime(time.Duration(time.Second))
		r.ObserveResponse(s.req, &BaseAttempt{Endpoint: e})
	}
	c.Assert(countB > countA*2, Equals, true)
}

// Make sure that failover avoids to hit the same endpoint
func (s *RoundRobinSuite) TestFailoverAvoidsSameEndpoint(c *C) {
	r := s.newRR()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	r.AddEndpoint(uA)
	r.AddEndpoint(uB)

	failedRequest := &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Endpoint: uA,
				Error:    fmt.Errorf("Something failed"),
			},
		},
	}

	u, err := r.NextEndpoint(failedRequest)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uB)
}

// Make sure that failover avoids to hit the same endpoints in case if there are multiple consequent failures
func (s *RoundRobinSuite) TestFailoverAvoidsSameEndpointMultipleFailures(c *C) {
	r := s.newRR()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	uC := MustParseUrl("http://localhost:5002")
	r.AddEndpoint(uA)
	r.AddEndpoint(uB)
	r.AddEndpoint(uC)

	failedRequest := &BaseRequest{
		Attempts: []Attempt{
			&BaseAttempt{
				Endpoint: uA,
				Error:    fmt.Errorf("Something failed"),
			},
			&BaseAttempt{
				Endpoint: uB,
				Error:    fmt.Errorf("Something failed"),
			},
		},
	}

	u, err := r.NextEndpoint(failedRequest)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uC)
}

// Removing endpoints from the load balancer works fine as well
func (s *RoundRobinSuite) TestRemoveMultipleEndpoints(c *C) {
	r := s.newRR()

	uA := MustParseUrl("http://localhost:5000")
	uB := MustParseUrl("http://localhost:5001")
	uC := MustParseUrl("http://localhost:5002")
	r.AddEndpoint(uA)
	r.AddEndpoint(uB)
	r.AddEndpoint(uC)

	u, err := r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uC)

	// There's only one endpoint left
	r.RemoveEndpoint(uA)
	r.RemoveEndpoint(uB)
	u, err = r.NextEndpoint(s.req)
	c.Assert(err, IsNil)
	c.Assert(u, Equals, uC)
}
