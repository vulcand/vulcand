package metrics

import (
	"math"
	"sort"
)

type CompareFn func(float64, float64) bool

func GTFloat64(a float64, b float64) bool {
	return a > b
}

func SplitFloat64(compare CompareFn, multiplier, sentinel float64, values []float64) (good map[float64]bool, bad map[float64]bool) {
	good, bad = make(map[float64]bool), make(map[float64]bool)
	var newValues []float64
	if len(values)%2 == 0 {
		newValues = make([]float64, len(values)+1)
		copy(newValues, values)
		// Add a sentinel endpoint so we can distinguish outliers better
		newValues[len(newValues)-1] = sentinel
	} else {
		newValues = values
	}

	m := median(newValues)
	mAbs := medianAbsoluteDeviation(newValues)
	for _, v := range values {
		if compare(v, m+mAbs*multiplier) {
			bad[v] = true
		} else {
			good[v] = true
		}
	}
	return good, bad
}

func median(values []float64) float64 {
	vals := make([]float64, len(values))
	copy(vals, values)
	sort.Float64s(vals)
	l := len(vals)
	if l%2 != 0 {
		return vals[l/2]
	} else {
		return (vals[l/2-1] + vals[l/2]) / 2.0
	}
}

func medianAbsoluteDeviation(values []float64) float64 {
	m := median(values)
	distances := make([]float64, len(values))
	for i, v := range values {
		distances[i] = math.Abs(v - m)
	}
	return median(distances)
}
