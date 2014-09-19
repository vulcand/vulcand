package server

import (
	"fmt"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/endpoint"
)

type Reporter struct {
	c         metrics.Client
	locPrefix string
}

func NewReporter(c metrics.Client, locationId string) *Reporter {
	return &Reporter{
		c:         c,
		locPrefix: escape(locationId),
	}
}

func (rp *Reporter) ObserveRequest(r request.Request) {
}

func (rp *Reporter) ObserveResponse(r request.Request, a request.Attempt) {
	if a == nil {
		return
	}
	rp.emitMetrics(metric("location", rp.locPrefix), r, a)
	if a.GetEndpoint() != nil {
		ve, ok := a.GetEndpoint().(*endpoint.VulcanEndpoint)
		if ok {
			rp.emitMetrics(metric("upstream", escape(ve.UpstreamId), escape(ve.Id)), r, a)
		}
	}
}

func (rp *Reporter) emitMetrics(p string, r request.Request, a request.Attempt) {
	// Report ttempt roundtrip time
	rp.c.TimingMs(metric(p, "roundtrip"), a.GetDuration(), 1)

	// Report request throughput
	if body := r.GetBody(); body != nil {
		if bytes, err := body.TotalSize(); err != nil {
			rp.c.Timing(metric(p, "request", "bytes"), bytes, 1)
		}
	}

	// Response code-related metrics
	if re := a.GetResponse(); re != nil {
		rp.c.Inc(metric(p, "code", fmt.Sprintf("%v", re.StatusCode)), 1, 1)
		rp.c.Inc(metric(p, "request"), 1, 1)

		if 200 <= re.StatusCode && re.StatusCode < 300 {
			rp.c.Inc(metric(p, "success"), 1, 1)
		} else {
			rp.c.Inc(metric(p, "failure"), 1, 1)
		}
	}
}

func escape(in string) string {
	return strings.Replace(in, ".", "_", -1)
}

func metric(p ...string) string {
	return strings.Join(p, ".")
}
