package scroll

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
)

type appStats struct {
	metrics metrics.Metrics
}

func newAppStats(metrics metrics.Metrics) *appStats {
	return &appStats{metrics}
}

func (s *appStats) TrackRequest(metricID string, status int, time time.Duration) {
	if s.metrics == nil {
		return
	}

	s.TrackRequestTime(metricID, time)
	s.TrackTotalRequests(metricID)
	if status != http.StatusOK {
		s.TrackFailedRequests(metricID, status)
	}
}

func (s *appStats) TrackRequestTime(metricID string, time time.Duration) {
	s.metrics.EmitTimer(fmt.Sprintf("api.%v.time", metricID), time)
}

func (s *appStats) TrackTotalRequests(metricID string) {
	s.metrics.EmitCounter(fmt.Sprintf("api.%v.count.total", metricID), 1)
}

func (s *appStats) TrackFailedRequests(metricID string, status int) {
	s.metrics.EmitCounter(fmt.Sprintf("api.%v.count.failed.%v", metricID, status), 1)
}
