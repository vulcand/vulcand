package server

import (
	"fmt"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

// Reporter reports real time metrics to the Statsd client
type Reporter struct {
	c         metrics.Client
	locPrefix string
}

func NewReporter(c metrics.Client, locationId string) *Reporter {
	return &Reporter{
		c:         c,
		locPrefix: locationId,
	}
}

func (rp *Reporter) ObserveRequest(r request.Request) {
}

func (rp *Reporter) ObserveResponse(r request.Request, a request.Attempt) {
	if a == nil {
		return
	}
	rp.emitMetrics(r, a, "location", rp.locPrefix)
	if a.GetEndpoint() != nil {
		ve, ok := a.GetEndpoint().(*muxEndpoint)
		if ok {
			rp.emitMetrics(r, a, "upstream", ve.location.Upstream.Id, ve.endpoint.Id)
		}
	}
}

func (rp *Reporter) emitMetrics(r request.Request, a request.Attempt, p ...string) {
	// Report ttempt roundtrip time
	m := rp.c.Metric(p...)
	rp.c.TimingMs(m.Metric("rtt"), a.GetDuration(), 1)

	// Report request throughput
	if body := r.GetBody(); body != nil {
		if bytes, err := body.TotalSize(); err != nil {
			rp.c.Timing(m.Metric("request", "bytes"), bytes, 1)
		}
	}

	// Response code-related metrics
	if re := a.GetResponse(); re != nil {
		rp.c.Inc(m.Metric("code", fmt.Sprintf("%v", re.StatusCode)), 1, 1)
		rp.c.Inc(m.Metric("request"), 1, 1)
	}
}

func escape(in string) string {
	return strings.Replace(in, ".", "_", -1)
}

func metric(p ...string) string {
	return strings.Join(p, ".")
}
