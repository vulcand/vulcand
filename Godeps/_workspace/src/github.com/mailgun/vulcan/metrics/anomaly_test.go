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
		good   map[float64]bool
		bad    map[float64]bool
	}{
		{
			values: []float64{0, 0},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{},
		},

		{
			values: []float64{0, 1},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{1: true},
		},
		{
			values: []float64{0.1, 0.1},
			good:   map[float64]bool{0.1: true},
			bad:    map[float64]bool{},
		},

		{
			values: []float64{0.15, 0.1},
			good:   map[float64]bool{0.15: true, 0.1: true},
			bad:    map[float64]bool{},
		},
		{
			values: []float64{0.01, 0.01},
			good:   map[float64]bool{0.01: true},
			bad:    map[float64]bool{},
		},
		{
			values: []float64{0.012, 0.01, 1},
			good:   map[float64]bool{0.012: true, 0.01: true},
			bad:    map[float64]bool{1: true},
		},
		{
			values: []float64{0, 0, 1, 1},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{1: true},
		},
		{
			values: []float64{0, 0.1, 0.1, 0},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{0.1: true},
		},
		{
			values: []float64{0, 0.01, 0.1, 0},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{0.01: true, 0.1: true},
		},
		{
			values: []float64{0, 0.01, 0.02, 1},
			good:   map[float64]bool{0: true, 0.01: true, 0.02: true},
			bad:    map[float64]bool{1: true},
		},
		{
			values: []float64{0, 0, 0, 0, 0, 0.01, 0.02, 1},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{0.01: true, 0.02: true, 1: true},
		},
	}
	for _, v := range vals {
		good, bad := SplitFloat64(1.5, 0, v.values)
		c.Assert(good, DeepEquals, v.good)
		c.Assert(bad, DeepEquals, v.bad)
	}
}

func (s *AnomalySuite) TestSplitFailRates(c *C) {
	vals := []struct {
		values []float64
		good   map[float64]bool
		bad    map[float64]bool
	}{
		{
			values: []float64{0, 0},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{},
		},

		{
			values: []float64{0, 1},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{1: true},
		},
		{
			values: []float64{0.1, 0.1},
			good:   map[float64]bool{0.1: true},
			bad:    map[float64]bool{},
		},

		{
			values: []float64{0.15, 0.1},
			good:   map[float64]bool{0.15: true, 0.1: true},
			bad:    map[float64]bool{},
		},
		{
			values: []float64{0.01, 0.01},
			good:   map[float64]bool{0.01: true},
			bad:    map[float64]bool{},
		},
		{
			values: []float64{0.012, 0.01, 1},
			good:   map[float64]bool{0.012: true, 0.01: true},
			bad:    map[float64]bool{1: true},
		},
		{
			values: []float64{0, 0, 1, 1},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{1: true},
		},
		{
			values: []float64{0, 0.1, 0.1, 0},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{0.1: true},
		},
		{
			values: []float64{0, 0.01, 0.1, 0},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{0.01: true, 0.1: true},
		},
		{
			values: []float64{0, 0.01, 0.02, 1},
			good:   map[float64]bool{0: true, 0.01: true, 0.02: true},
			bad:    map[float64]bool{1: true},
		},
		{
			values: []float64{0, 0, 0, 0, 0, 0.01, 0.02, 1},
			good:   map[float64]bool{0: true},
			bad:    map[float64]bool{0.01: true, 0.02: true, 1: true},
		},
	}
	for _, v := range vals {
		good, bad := SplitRatios(v.values)
		c.Assert(good, DeepEquals, v.good)
		c.Assert(bad, DeepEquals, v.bad)
	}
}

func (s *AnomalySuite) TestSplitLatencies(c *C) {
	vals := []struct {
		values []time.Duration
		good   map[time.Duration]bool
		bad    map[time.Duration]bool
	}{
		{
			values: []time.Duration{0, 0},
			good:   map[time.Duration]bool{0: true},
			bad:    map[time.Duration]bool{},
		},
		{
			values: []time.Duration{time.Millisecond, 2 * time.Millisecond},
			good:   map[time.Duration]bool{time.Millisecond: true, 2 * time.Millisecond: true},
			bad:    map[time.Duration]bool{},
		},
		{
			values: []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond},
			good:   map[time.Duration]bool{time.Millisecond: true, 2 * time.Millisecond: true},
			bad:    map[time.Duration]bool{4 * time.Millisecond: true},
		},
		{
			values: []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond, 40 * time.Millisecond},
			good:   map[time.Duration]bool{time.Millisecond: true, 2 * time.Millisecond: true, 4 * time.Millisecond: true},
			bad:    map[time.Duration]bool{40 * time.Millisecond: true},
		},
		{
			values: []time.Duration{40 * time.Millisecond, 60 * time.Millisecond, time.Second},
			good:   map[time.Duration]bool{40 * time.Millisecond: true, 60 * time.Millisecond: true},
			bad:    map[time.Duration]bool{time.Second: true},
		},
	}
	for _, v := range vals {
		good, bad := SplitLatencies(v.values, time.Millisecond)
		c.Assert(good, DeepEquals, v.good)
		c.Assert(bad, DeepEquals, v.bad)
	}
}
