package frontend

import (
	"io"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/utils"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

// DefaultHandler default error handler.
var DefaultHandler utils.ErrorHandler = &StdHandler{}
var logLimiter = rate.NewLimiter(rate.Every(time.Second), 10)

// StdHandler Standard error handler.
type StdHandler struct{}

func (e *StdHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, err error) {
	statusCode := http.StatusInternalServerError

	if e, ok := err.(net.Error); ok {
		if e.Timeout() {
			statusCode = http.StatusGatewayTimeout
		} else {
			statusCode = http.StatusBadGateway
		}
	} else if err == io.EOF {
		statusCode = http.StatusBadGateway
	} else if err == context.Canceled {
		statusCode = utils.StatusClientClosedRequest
	} else if err.Error() == "no servers in the pool" {
		statusCode = http.StatusServiceUnavailable
		if logLimiter.Allow() {
			log.WithFields(log.Fields{
				"url":         req.URL.String(),
				"log-limiter": int(logLimiter.Tokens()),
			}).Warnf("request failed with 503; the backend has no servers")
		}
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(statusText(statusCode)))
	log.Debugf("'%d %s' caused by: %v", statusCode, statusText(statusCode), err)
}

func statusText(statusCode int) string {
	if statusCode == utils.StatusClientClosedRequest {
		return utils.StatusClientClosedRequestText
	}
	return http.StatusText(statusCode)
}
