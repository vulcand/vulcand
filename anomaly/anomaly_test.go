package anomaly

import (
	"fmt"
	"testing"
	"time"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
)

func TestAnomaly(t *testing.T) { TestingT(t) }

type AnomalySuite struct {
}

var _ = Suite(&AnomalySuite{})

func (s *AnomalySuite) TestMarkEmptyDoesNotCrash(c *C) {
	var endpoints []*Endpoint
	MarkEndpointAnomalies(endpoints)

	var stats []*RoundTripStats
	MarkAnomalies(stats)
}

func (s *AnomalySuite) TestMarkAnomalies(c *C) {
	tc := []struct {
		Endpoints []*Endpoint
		Verdicts  []Verdict
	}{
		{
			Endpoints: []*Endpoint{
				&Endpoint{
					Stats: RoundTripStats{},
				},
			},
			Verdicts: []Verdict{{IsBad: false}},
		},
		{
			Endpoints: []*Endpoint{
				&Endpoint{
					Stats: RoundTripStats{
						Counters: Counters{
							Period:    time.Second,
							NetErrors: 10,
							Total:     100,
						},
					},
				},
				&Endpoint{
					Stats: RoundTripStats{
						Counters: Counters{
							Period:    time.Second,
							NetErrors: 0,
							Total:     100,
						},
					},
				},
			},
			Verdicts: []Verdict{{IsBad: true, Anomalies: []Anomaly{{Code: CodeNetErrorRate, Message: MessageNetErrRate}}}, {}},
		},
		{
			Endpoints: []*Endpoint{
				&Endpoint{
					Stats: RoundTripStats{
						Counters: Counters{
							Period:      time.Second,
							Total:       100,
							StatusCodes: []StatusCode{{Code: 500, Count: 10}, {Code: 200, Count: 90}},
						},
					},
				},
				&Endpoint{
					Stats: RoundTripStats{
						Counters: Counters{
							Period:    time.Second,
							NetErrors: 0,
							Total:     100,
						},
					},
				},
			},
			Verdicts: []Verdict{{IsBad: true, Anomalies: []Anomaly{{Code: CodeAppErrorRate, Message: MessageAppErrRate}}}, {}},
		},
		{
			Endpoints: []*Endpoint{
				&Endpoint{
					Stats: RoundTripStats{
						Counters: Counters{
							Period:      time.Second,
							Total:       100,
							StatusCodes: []StatusCode{{Code: 500, Count: 10}, {Code: 200, Count: 90}},
						},
					},
				},
				&Endpoint{
					Stats: RoundTripStats{
						Counters: Counters{
							Period:    time.Second,
							NetErrors: 0,
							Total:     100,
						},
					},
				},
			},
			Verdicts: []Verdict{{IsBad: true, Anomalies: []Anomaly{{Code: CodeAppErrorRate, Message: MessageAppErrRate}}}, {}},
		},
		{
			Endpoints: []*Endpoint{
				&Endpoint{
					Stats: RoundTripStats{
						LatencyBrackets: []Bracket{
							{
								Quantile: 50,
								Value:    time.Second,
							},
						},
					},
				},
				&Endpoint{
					Stats: RoundTripStats{
						LatencyBrackets: []Bracket{
							{
								Quantile: 50,
								Value:    time.Millisecond,
							},
						},
					},
				},
			},
			Verdicts: []Verdict{{IsBad: true, Anomalies: []Anomaly{{Code: CodeLatency, Message: fmt.Sprintf(MessageLatency, 50.0)}}}, {}},
		},
	}

	for _, t := range tc {
		MarkEndpointAnomalies(t.Endpoints)
		for i, e := range t.Endpoints {
			c.Assert(e.Stats.Verdict, DeepEquals, t.Verdicts[i])
		}
	}
}
