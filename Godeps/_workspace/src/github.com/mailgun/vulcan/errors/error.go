// Utility functions for producing errorneous http responses
package errors

import (
	"encoding/json"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"net/http"
)

const (
	StatusTooManyRequests = 429
)

type ProxyError interface {
	GetStatusCode() int
	Error() string
}

type Formatter interface {
	Format(ProxyError) (statusCode int, body []byte, contentType string)
}

type JsonFormatter struct {
}

func (f *JsonFormatter) Format(err ProxyError) (int, []byte, string) {
	encodedError, e := json.Marshal(map[string]interface{}{
		"error": string(err.Error()),
	})
	if e != nil {
		log.Errorf("Failed to serialize: %s", e)
		encodedError = []byte("{}")
	}
	return err.GetStatusCode(), encodedError, "application/json"
}

type HttpError struct {
	StatusCode int
	Body       string
}

func FromStatus(statusCode int) *HttpError {
	return &HttpError{statusCode, http.StatusText(statusCode)}
}

func (r *HttpError) Error() string {
	return r.Body
}

func (r *HttpError) GetStatusCode() int {
	return r.StatusCode
}
