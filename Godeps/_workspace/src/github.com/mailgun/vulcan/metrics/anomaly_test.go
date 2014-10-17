package metrics

import (
	"time"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

type AnomalySuite struct {
}

var _ = Suite(&AnomalySuite{})

func (s *AnomalySuite) TestMedian(c *C) {
	c.Assert(median([]float64{0.1, 0.2}), Equals, (float64(0.1)+float64(0.2))/2.0)
	c.Assert(median([]float64{0.3, 0.2, 0.5}), Equals, 0.3)
}

func (s *AnomalySuite) TestSplitRatios(c *C) {
	vals := []struct {
		values []float64
		good   []float64
		bad    []float64
	}{
		{
			values: []float64{0, 0},
			good:   []float64{0},
			bad:    []float64{},
		},

		{
			values: []float64{0, 1},
			good:   []float64{0},
			bad:    []float64{1},
		},
		{
			values: []float64{0.1, 0.1},
			good:   []float64{0.1},
			bad:    []float64{},
		},

		{
			values: []float64{0.15, 0.1},
			good:   []float64{0.15, 0.1},
			bad:    []float64{},
		},
		{
			values: []float64{0.01, 0.01},
			good:   []float64{0.01},
			bad:    []float64{},
		},
		{
			values: []float64{0.012, 0.01, 1},
			good:   []float64{0.012, 0.01},
			bad:    []float64{1},
		},
		{
			values: []float64{0, 0, 1, 1},
			good:   []float64{0},
			bad:    []float64{1},
		},
		{
			values: []float64{0, 0.1, 0.1, 0},
			good:   []float64{0},
			bad:    []float64{0.1},
		},
		{
			values: []float64{0, 0.01, 0.1, 0},
			good:   []float64{0},
			bad:    []float64{0.01, 0.1},
		},
		{
			values: []float64{0, 0.01, 0.02, 1},
			good:   []float64{0, 0.01, 0.02},
			bad:    []float64{1},
		},
		{
			values: []float64{0, 0, 0, 0, 0, 0.01, 0.02, 1},
			good:   []float64{0},
			bad:    []float64{0.01, 0.02, 1},
		},
	}
	for _, v := range vals {
		good, bad := SplitRatios(v.values)
		vgood, vbad := make(map[float64]bool, len(v.good)), make(map[float64]bool, len(v.bad))
		for _, v := range v.good {
			vgood[v] = true
		}
		for _, v := range v.bad {
			vbad[v] = true
		}

		c.Assert(good, DeepEquals, vgood)
		c.Assert(bad, DeepEquals, vbad)
	}
}

func (s *AnomalySuite) TestSplitLatencies(c *C) {
	vals := []struct {
		values []time.Duration
		good   []time.Duration
		bad    []time.Duration
	}{
		{
			values: []time.Duration{0, 0},
			good:   []time.Duration{0},
			bad:    []time.Duration{},
		},
		{
			values: []time.Duration{time.Millisecond, 2 * time.Millisecond},
			good:   []time.Duration{time.Millisecond, 2 * time.Millisecond},
			bad:    []time.Duration{},
		},
		{
			values: []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond},
			good:   []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond},
			bad:    []time.Duration{},
		},
		{
			values: []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond, 40 * time.Millisecond},
			good:   []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond},
			bad:    []time.Duration{40 * time.Millisecond},
		},
		{
			values: []time.Duration{40 * time.Millisecond, 60 * time.Millisecond, time.Second},
			good:   []time.Duration{40 * time.Millisecond, 60 * time.Millisecond},
			bad:    []time.Duration{time.Second},
		},
	}
	for _, v := range vals {
		good, bad := SplitLatencies(v.values, time.Millisecond)

		vgood, vbad := make(map[time.Duration]bool, len(v.good)), make(map[time.Duration]bool, len(v.bad))
		for _, v := range v.good {
			vgood[v] = true
		}
		for _, v := range v.bad {
			vbad[v] = true
		}

		c.Assert(good, DeepEquals, vgood)
		c.Assert(bad, DeepEquals, vbad)
	}
}
