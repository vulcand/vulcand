package roundrobin

import (
	"fmt"
	"time"

	"github.com/mailgun/timetools"
	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/metrics"
	. "gopkg.in/check.v1"
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

func (s *FSMSuite) newF(endpoints []*WeightedEndpoint) *FSMHandler {
	o, err := NewFSMHandlerWithOptions(s.tm)
	if err != nil {
		panic(err)
	}
	o.Init(endpoints)
	return o
}

func (s *FSMSuite) advanceTime(d time.Duration) {
	s.tm.CurrentTime = s.tm.CurrentTime.Add(d)
}

// Check our special greater function that neglects insigificant differences
func (s *FSMSuite) TestFSMSplit(c *C) {
	vals := []struct {
		endpoints []*WeightedEndpoint
		good      []int
		bad       []int
	}{
		{
			endpoints: newW(0, 0),
			good:      []int{0, 1},
			bad:       []int{},
		},
		{
			endpoints: newW(0, 1),
			good:      []int{0},
			bad:       []int{1},
		},
		{
			endpoints: newW(0.1, 0.1),
			good:      []int{0, 1},
			bad:       []int{},
		},
		{
			endpoints: newW(0.15, 0.1),
			good:      []int{0, 1},
			bad:       []int{},
		},
		{
			endpoints: newW(0.01, 0.01),
			good:      []int{0, 1},
			bad:       []int{},
		},
		{
			endpoints: newW(0.012, 0.01, 1),
			good:      []int{0, 1},
			bad:       []int{2},
		},
		{
			endpoints: newW(0, 0, 1, 1),
			good:      []int{0, 1},
			bad:       []int{2, 3},
		},
		{
			endpoints: newW(0, 0.1, 0.1, 0),
			good:      []int{0, 3},
			bad:       []int{1, 2},
		},
		{
			endpoints: newW(0, 0.01, 0.1, 0),
			good:      []int{0, 3},
			bad:       []int{1, 2},
		},
		{
			endpoints: newW(0, 0.01, 0.02, 1),
			good:      []int{0, 1, 2},
			bad:       []int{3},
		},
		{
			endpoints: newW(0, 0, 0, 0, 0, 0.01, 0.02, 1),
			good:      []int{0, 1, 2, 3, 4},
			bad:       []int{5, 6, 7},
		},
	}
	for _, v := range vals {
		good, bad := splitEndpoints(v.endpoints)
		for _, id := range v.good {
			c.Assert(good[fmt.Sprintf("http://localhost:500%d", id)], Equals, true)
		}
		for _, id := range v.bad {
			c.Assert(bad[fmt.Sprintf("http://localhost:500%d", id)], Equals, true)
		}
	}
}

func (s *FSMSuite) TestInvalidParameters(c *C) {
	_, err := NewFSMHandlerWithOptions(nil)
	c.Assert(err, NotNil)
}

func (s *FSMSuite) TestNoEndpoints(c *C) {
	adjusted, err := s.newF(newW()).AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 0)
}

func (s *FSMSuite) TestOneEndpoint(c *C) {
	adjusted, err := s.newF(newW(1)).AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1})
}

func (s *FSMSuite) TestAllEndpointsAreGood(c *C) {
	adjusted, err := s.newF(newW(0, 0)).AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, 1})
}

func (s *FSMSuite) TestAllEndpointsAreBad(c *C) {
	adjusted, err := s.newF(newW(0.13, 0.14, 0.14)).AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, 1, 1})
}

func (s *FSMSuite) TestMetricsAreNotReady(c *C) {
	endpoints := []*WeightedEndpoint{
		&WeightedEndpoint{
			meter:           &metrics.TestMeter{Rate: 0.5, NotReady: true},
			endpoint:        endpoint.MustParseUrl("http://localhost:5000"),
			weight:          1,
			effectiveWeight: 1,
		},
		&WeightedEndpoint{
			meter:           &metrics.TestMeter{Rate: 0, NotReady: true},
			endpoint:        endpoint.MustParseUrl("http://localhost:5001"),
			weight:          1,
			effectiveWeight: 1,
		},
	}
	adjusted, err := s.newF(endpoints).AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, 1})
}

func (s *FSMSuite) TestWeightIncrease(c *C) {
	endpoints := newW(0.5, 0)
	f := s.newF(endpoints)

	adjusted, err := f.AdjustWeights()

	// It will adjust weights and set timer
	c.Assert(err, IsNil)
	c.Assert(len(adjusted), Equals, 2)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, FSMGrowFactor})
	for _, a := range adjusted {
		a.GetEndpoint().setEffectiveWeight(a.GetWeight())
	}

	// We will wait some time until we gather some stats
	adjusted, err = f.AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, FSMGrowFactor})

	// As time passes, let's repeat this procedure to see if we hit the ceiling
	for i := 0; i < 6; i += 1 {
		adjusted, err := f.AdjustWeights()
		c.Assert(err, IsNil)
		for _, a := range adjusted {
			a.GetEndpoint().setEffectiveWeight(a.GetWeight())
		}
		s.advanceTime(endpoints[0].meter.GetWindowSize()/2 + time.Second)
	}

	// Algo has not changed the weight of the bad endpoint
	c.Assert(endpoints[0].GetEffectiveWeight(), Equals, 1)
	// Algo has adjusted the weight of the good endpoint to the maximum number
	c.Assert(endpoints[1].GetEffectiveWeight(), Equals, FSMMaxWeight)
}

func (s *FSMSuite) TestRevert(c *C) {
	endpoints := newW(0.5, 0)
	f := s.newF(endpoints)

	bad := endpoints[0]
	adjusted, err := f.AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, FSMGrowFactor})
	for _, a := range adjusted {
		a.GetEndpoint().setEffectiveWeight(a.GetWeight())
	}

	// The situation have recovered, so FSM will try to bring back the bad endpoint into life by reverting the weights back
	s.advanceTime(endpoints[0].meter.GetWindowSize()/2 + time.Second)
	bad.GetMeter().(*metrics.TestMeter).Rate = 0
	f.AdjustWeights()

	adjusted, err = f.AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, 1})
}

// Case when the increasing weights went wrong and the good endpoints started failing
func (s *FSMSuite) TestProbingUnsuccessfull(c *C) {
	endpoints := newW(0.5, 0.5, 0, 0, 0)
	f := s.newF(endpoints)

	adjusted, err := f.AdjustWeights()

	// It will adjust weight and set timer
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, 1, FSMGrowFactor, FSMGrowFactor, FSMGrowFactor})
	for _, a := range adjusted {
		a.GetEndpoint().setEffectiveWeight(a.GetWeight())
	}
	// Times has passed and good endpoint appears to behave worse now, oh no!
	for _, e := range endpoints {
		e.GetMeter().(*metrics.TestMeter).Rate = 0.5
	}
	s.advanceTime(endpoints[0].meter.GetWindowSize()/2 + time.Second)

	// As long as all endpoints are equally bad now, we will revert weights back
	adjusted, err = f.AdjustWeights()
	c.Assert(err, IsNil)
	c.Assert(getWeights(adjusted), DeepEquals, []int{1, 1, 1, 1, 1})
}

func (s *FSMSuite) TestNormalize(c *C) {
	weights := newWeights(1, 2, 3, 4)
	c.Assert(weights, DeepEquals, normalizeWeights(weights))
	c.Assert(newWeights(1, 1, 1, 4), DeepEquals, normalizeWeights(newWeights(4, 4, 4, 16)))
}

func newW(failRates ...float64) []*WeightedEndpoint {
	out := make([]*WeightedEndpoint, len(failRates))
	for i, r := range failRates {
		out[i] = &WeightedEndpoint{
			meter:           &metrics.TestMeter{Rate: r, WindowSize: time.Second * 10},
			endpoint:        endpoint.MustParseUrl(fmt.Sprintf("http://localhost:500%d", i)),
			weight:          1,
			effectiveWeight: 1,
		}
	}
	return out
}

func getWeights(weights []SuggestedWeight) []int {
	out := make([]int, len(weights))
	for i, w := range weights {
		out[i] = w.GetWeight()
	}
	return out
}

func newWeights(weights ...int) []SuggestedWeight {
	out := make([]SuggestedWeight, len(weights))
	for i, w := range weights {
		out[i] = &EndpointWeight{
			Weight:   w,
			Endpoint: &WeightedEndpoint{endpoint: endpoint.MustParseUrl(fmt.Sprintf("http://localhost:500%d", i))},
		}
	}
	return out
}
