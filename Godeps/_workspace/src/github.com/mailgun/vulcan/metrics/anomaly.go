package metrics

import (
	"math"
	"sort"
)

// SplitFloat64 provides simple anomaly detection for skewed data sets with no particular distribution.
// In essense it applies the formula if(v > median(values) + threshold * medianAbsoluteDeviation) -> anomaly
// There's a corner case where there are just 2 values, so by definition there's no value that exceeds the threshold.
// This case is solved by introducing additional value that we know is good, e.g. 0. That helps to improve the detection results
// on such data sets.
func SplitFloat64(threshold, sentinel float64, values []float64) (good map[float64]bool, bad map[float64]bool) {
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
		if v > m+mAbs*threshold {
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
	}
	return (vals[l/2-1] + vals[l/2]) / 2.0
}

func medianAbsoluteDeviation(values []float64) float64 {
	m := median(values)
	distances := make([]float64, len(values))
	for i, v := range values {
		distances[i] = math.Abs(v - m)
	}
	return median(distances)
}
