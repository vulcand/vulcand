package roundrobin

import (
	"fmt"
	timetools "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/metrics"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/launchpad.net/gocheck"
	"time"
)

type FSMSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&FSMSuite{})

func (s *FSMSuite) SetUpTest(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *FSMSuite) newF() *FSMHandler {
	o, err := NewFSMHandlerWithOptions(s.tm, FSMDefaultProbingPeriod)
	if err != nil {
		panic(err)
	}
	return o
}

func (s *FSMSuite) advanceTime(d time.Duration) {
	s.tm.CurrentTime = s.tm.CurrentTime.Add(d)
}

// Check our special greater function that neglects insigificant differences
func (s *FSMSuite) TestFSMGreater(c *C) {
	vals := []struct {
		a        float64
		b        float64
		expected bool
	}{
		{0, 0, false},
		{0.1, 0.1, false},
		{0.15, 0.1, false},
		{0.01, 0.01, false},
		{0.012, 0.01, false},
		{0.2, 0.1, true},
		{0.51, 0.1, true},
	}
	for _, v := range vals {
		c.Assert(greater(v.a, v.b), Equals, v.expected)
	}
}

func (s *FSMSuite) TestInvalidParameters(c *C) {
	_, err := NewFSMHandlerWithOptions(nil, FSMDefaultProbingPeriod)
	c.Assert(err, NotNil)

	_, err = NewFSMHandlerWithOptions(s.tm, time.Millisecond)
	c.Assert(err, NotNil)
}

func (s *FSMSuite) TestNoEndpoints(c *C) {
	adjusted, err := s.newF().AdjustWeights(newW())
	c.Assert(err, IsNil)
	c.Assert(adjusted, IsNil)
}

func (s *FSMSuite) TestOneEndpoint(c *C) {
	adjusted, err := s.newF().AdjustWeights(newW(1))
	c.Assert(err, IsNil)
	c.Assert(adjusted, IsNil)

	adjusted, err = s.newF().AdjustWeights(newW(0))
	c.Assert(err, IsNil)
	c.Assert(adjusted, IsNil)
}

func (s *FSMSuite) TestAllEndpointsAreGood(c *C) {
	adjusted, err := s.newF().AdjustWeights(newW(0, 0))
	c.Assert(err, IsNil)
	c.Assert(adjusted, IsNil)
}

func (s *FSMSuite) TestAllEndpointsAreBad(c *C) {
	adjusted, err := s.newF().AdjustWeights(newW(0.13, 0.14))
	c.Assert(err, IsNil)
	c.Assert(adjusted, IsNil)
}

func (s *FSMSuite) TestMetricsAreNotReady(c *C) {
	endpoints := []*WeightedEndpoint{
		&WeightedEndpoint{
			failRateMeter:   &metrics.TestMeter{Rate: 0.5, NotReady: true},
			endpoint:        endpoint.MustParseUrl("http://localhost:5000"),
			weight:          1,
			effectiveWeight: 1,
		},
		&WeightedEndpoint{
			failRateMeter:   &metrics.TestMeter{Rate: 0, NotReady: true},
			endpoint:        endpoint.MustParseUrl("http://localhost:5001"),
			weight:          1,
			effectiveWeight: 1,
		},
	}
	adjusted, err := s.newF().AdjustWeights(endpoints)
	c.Assert(err, IsNil)
	c.Assert(adjusted, IsNil)
}

func (s *FSMSuite) TestWeightIncrease(c *C) {
	f := s.newF()
	endpoints := newW(0.5, 0)
	good := endpoints[1]
	adjusted, err := f.AdjustWeights(endpoints)

	// It will adjust weight and enter probing state
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 1)
	c.Assert(f.GetState(), DeepEquals, FSMState(FSMProbing))
	c.Assert(adjusted[0].GetWeight(), Equals, FSMGrowFactor)

	// We've increased weight of the "better endpoint"
	c.Assert(adjusted[0].GetEndpoint(), Equals, good)

	// Let's actually apply the weight
	adjusted[0].GetEndpoint().setEffectiveWeight(adjusted[0].GetWeight())

	// Probing state will wait some time until we gather some stats
	adjusted, err = f.AdjustWeights(endpoints)
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 0)

	// Time has passed and we have commited the weights and went back to start
	s.advanceTime(FSMDefaultProbingPeriod + time.Second)
	adjusted, err = f.AdjustWeights(endpoints)
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 0)
	c.Assert(f.GetState(), Equals, FSMState(FSMStart))

	// As time passes, let's repeat this procedure to see if we hit the ceiling
	for i := 0; i < 6; i += 1 {
		adjusted, err := f.AdjustWeights(endpoints)
		c.Assert(err, IsNil)
		if len(adjusted) != 0 {
			for _, a := range adjusted {
				a.GetEndpoint().setEffectiveWeight(a.GetWeight())
			}
		}
		s.advanceTime(FSMDefaultProbingPeriod + time.Second)
	}

	// Algo has not changed the weight of the bad endpoint
	c.Assert(endpoints[0].GetEffectiveWeight(), Equals, 1)
	// Algo has adjusted the weight of the good endpoint to the maximum number
	c.Assert(endpoints[1].GetEffectiveWeight(), Equals, FSMMaxWeight)
}

func (s *FSMSuite) TestRevert(c *C) {
	f := s.newF()
	endpoints := newW(0.5, 0)
	bad, good := endpoints[0], endpoints[1]

	adjusted, err := f.AdjustWeights(endpoints)

	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 1)
	c.Assert(adjusted[0].GetWeight(), Equals, FSMGrowFactor)
	// Apply the suggested changes

	adjusted[0].GetEndpoint().setEffectiveWeight(adjusted[0].GetWeight())
	// Make sure we've commited the changes and went back to the start state
	s.advanceTime(FSMDefaultProbingPeriod + time.Second)
	f.AdjustWeights(endpoints)
	c.Assert(f.GetState(), Equals, FSMState(FSMStart))

	// The situation have recovered, so FSM will try to bring back the bad endpoint into life by reverting the weights back
	c.Assert(good.GetEffectiveWeight(), Equals, FSMGrowFactor)
	bad.GetMeter().(*metrics.TestMeter).Rate = 0

	adjusted, err = f.AdjustWeights(endpoints)
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 1)
	adjusted[0].GetEndpoint().setEffectiveWeight(adjusted[0].GetWeight())
	c.Assert(good.GetEffectiveWeight(), Equals, 1)
	c.Assert(bad.GetEffectiveWeight(), Equals, 1)
	c.Assert(f.GetState(), Equals, FSMState(FSMRevert))

	// We've successfully returned back to the original state
	s.advanceTime(FSMDefaultProbingPeriod + time.Second)
	adjusted, err = f.AdjustWeights(endpoints)
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 0)
	c.Assert(f.GetState(), Equals, FSMState(FSMStart))
}

// Case when the probing went wrong
func (s *FSMSuite) TestProbingUnsuccessfull(c *C) {
	f := s.newF()
	endpoints := newW(0.5, 0.01)
	good := endpoints[1]
	adjusted, err := f.AdjustWeights(endpoints)

	// It will adjust weight and enter probing state
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 1)
	c.Assert(f.GetState(), Equals, FSMState(FSMProbing))
	c.Assert(adjusted[0].GetWeight(), Equals, FSMGrowFactor)

	// We've increased weight of the "better endpoint"
	c.Assert(adjusted[0].GetEndpoint(), Equals, good)
	adjusted[0].GetEndpoint().setEffectiveWeight(adjusted[0].GetWeight())

	// Times has passed and good endpoint appears to behave worse now, oh no!
	good.GetMeter().(*metrics.TestMeter).Rate = 0.2
	s.advanceTime(FSMDefaultProbingPeriod + time.Second)

	// We will go to rollback state now and revert the weights
	adjusted, err = f.AdjustWeights(endpoints)
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 1)
	c.Assert(adjusted[0].GetEndpoint(), Equals, good)
	c.Assert(adjusted[0].GetWeight(), Equals, 1)
	adjusted[0].GetEndpoint().setEffectiveWeight(adjusted[0].GetWeight())
	c.Assert(f.GetState(), Equals, FSMState(FSMRollback))

	// We are in the rollback state until time passes
	adjusted, err = f.AdjustWeights(endpoints)
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 0)
	c.Assert(f.GetState(), Equals, FSMState(FSMRollback))

	// Time has passed and we have commited the weights and went back to start
	s.advanceTime(FSMDefaultProbingPeriod + time.Second)
	adjusted, err = f.AdjustWeights(endpoints)
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 0)
	c.Assert(f.GetState(), Equals, FSMState(FSMStart))
}

func newW(failRates ...float64) []*WeightedEndpoint {
	out := make([]*WeightedEndpoint, len(failRates))
	for i, r := range failRates {
		out[i] = &WeightedEndpoint{
			failRateMeter:   &metrics.TestMeter{Rate: r},
			endpoint:        endpoint.MustParseUrl(fmt.Sprintf("http://localhost:500%d", i)),
			weight:          1,
			effectiveWeight: 1,
		}
	}
	return out
}
