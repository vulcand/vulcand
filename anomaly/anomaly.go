package anomaly

import (
	"fmt"
	"time"

	"github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcand/backend"
)

const (
	CodeLatency = iota + 1
	CodeNetErrorRate
	CodeAppErrorRate
)

const (
	MessageNetErrRate = "Error rate stands out"
	MessageAppErrRate = "App error rate (status 500) stands out"
	MessageLatency    = "%0.2f quantile latency stands out"
)

// MarkEndpointAnomalies takes the list of endpoints and marks anomalies detected within this set
// by modifying the inner Verdict property.
func MarkEndpointAnomalies(endpoints []*backend.Endpoint) error {
	if len(endpoints) == 0 {
		return nil
	}

	stats := make([]*backend.RoundTripStats, len(endpoints))
	for i, e := range endpoints {
		stats[i] = &e.Stats
	}
	return MarkAnomalies(stats)
}

// MarkAnomalies takes the list of stats and marks anomalies detected within this group by updating
// the Verdict property.
func MarkAnomalies(stats []*backend.RoundTripStats) error {
	if len(stats) == 0 {
		return nil
	}
	if err := markLatencies(stats); err != nil {
		return err
	}
	if err := markNetErrorRates(stats); err != nil {
		return err
	}
	return markAppErrorRates(stats)
}

func markNetErrorRates(stats []*backend.RoundTripStats) error {
	errRates := make([]float64, len(stats))
	for i, s := range stats {
		errRates[i] = s.NetErrorRatio()
	}

	_, bad := metrics.SplitRatios(errRates)
	for _, s := range stats {
		if bad[s.NetErrorRatio()] {
			s.Verdict.IsBad = true
			s.Verdict.Anomalies = append(s.Verdict.Anomalies, backend.Anomaly{Code: CodeNetErrorRate, Message: MessageNetErrRate})
		}
	}
	return nil
}

func markLatencies(stats []*backend.RoundTripStats) error {
	// We are processing only median as others are more volatile
	return markLatency(0, stats)
}

func markLatency(index int, stats []*backend.RoundTripStats) error {
	quantiles := make([]time.Duration, len(stats))
	for i, s := range stats {
		v, err := s.LatencyBrackets.GetQuantile(50)
		if err != nil {
			return err
		}
		quantiles[i] = v.Value
	}

	quantile := stats[0].LatencyBrackets[index].Quantile
	_, bad := metrics.SplitLatencies(quantiles, time.Millisecond)
	for _, s := range stats {
		if bad[s.LatencyBrackets[index].Value] {
			s.Verdict.IsBad = true
			s.Verdict.Anomalies = append(
				s.Verdict.Anomalies,
				backend.Anomaly{
					Code:    CodeLatency,
					Message: fmt.Sprintf(MessageLatency, quantile),
				})
		}
	}
	return nil
}

func markAppErrorRates(stats []*backend.RoundTripStats) error {
	errRates := make([]float64, len(stats))
	for i, s := range stats {
		errRates[i] = s.AppErrorRatio()
	}

	_, bad := metrics.SplitRatios(errRates)
	for _, s := range stats {
		if bad[s.AppErrorRatio()] {
			s.Verdict.IsBad = true
			s.Verdict.Anomalies = append(
				s.Verdict.Anomalies, backend.Anomaly{Code: CodeAppErrorRate, Message: MessageAppErrRate})
		}
	}
	return nil
}
