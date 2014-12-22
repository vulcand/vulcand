package circuitbreaker

import (
	"fmt"
	"time"

	"github.com/mailgun/log"
	"github.com/mailgun/predicate"
	"github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcan/threshold"
)

// MustParseExpresison calls ParseExpression and panics if expression is incorrect, for use in tests
func MustParseExpression(in string) threshold.Predicate {
	e, err := ParseExpression(in)
	if err != nil {
		panic(err)
	}
	return e
}

// ParseExpression parses expression in the go language into predicates.
func ParseExpression(in string) (threshold.Predicate, error) {
	p, err := predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: threshold.AND,
			OR:  threshold.OR,
			EQ:  threshold.EQ,
			NEQ: threshold.NEQ,
			LT:  threshold.LT,
			LE:  threshold.LE,
			GT:  threshold.GT,
			GE:  threshold.GE,
		},
		Functions: map[string]interface{}{
			"LatencyAtQuantileMS": latencyAtQuantile,
			"NetworkErrorRatio":   networkErrorRatio,
			"ResponseCodeRatio":   responseCodeRatio,
		},
	})
	if err != nil {
		return nil, err
	}
	out, err := p.Parse(in)
	if err != nil {
		return nil, err
	}
	pr, ok := out.(threshold.Predicate)
	if !ok {
		return nil, fmt.Errorf("expected predicate, got %T", out)
	}
	return pr, nil
}

func latencyAtQuantile(quantile float64) threshold.RequestToInt {
	return func(r request.Request) int {
		m := getMetrics(r)
		if m == nil {
			return 0
		}
		h, err := m.GetLatencyHistogram()
		if err != nil {
			log.Errorf("Failed to get latency histogram, for %v error: %v", r, err)
			return 0
		}
		return int(h.LatencyAtQuantile(quantile) / time.Millisecond)
	}
}

func networkErrorRatio() threshold.RequestToFloat64 {
	return func(r request.Request) float64 {
		m := getMetrics(r)
		if m == nil {
			return 0
		}
		return m.GetNetworkErrorRatio()
	}
}

func responseCodeRatio(startA, endA, startB, endB int) threshold.RequestToFloat64 {
	return func(r request.Request) float64 {
		m := getMetrics(r)
		if m == nil {
			return 0
		}
		return m.GetResponseCodeRatio(startA, endA, startB, endB)
	}
}

func getMetrics(r request.Request) *metrics.RoundTripMetrics {
	m, ok := r.GetUserData(cbreakerMetrics)
	if !ok {
		return nil
	}
	return m.(*metrics.RoundTripMetrics)
}
