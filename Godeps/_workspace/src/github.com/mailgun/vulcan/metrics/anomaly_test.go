package metrics

import (
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

type AnomalySuite struct {
}

var _ = Suite(&AnomalySuite{})

func (s *AnomalySuite) TestSplitValues(c *C) {
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
		good, bad := SplitFloat64(GTFloat64, 1.5, 0, v.values)
		c.Assert(good, DeepEquals, v.good)
		c.Assert(bad, DeepEquals, v.bad)
	}
}

func (s *AnomalySuite) TestMedian(c *C) {
	c.Assert(median([]float64{0.1, 0.2}), Equals, (float64(0.1)+float64(0.2))/2.0)
	c.Assert(median([]float64{0.3, 0.2, 0.5}), Equals, 0.3)
}
