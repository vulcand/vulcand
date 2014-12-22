// Utility functions for producing errorneous http responses
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mailgun/log"
)

const (
	StatusTooManyRequests = 429
)

type ProxyError interface {
	GetStatusCode() int
	Error() string
	Headers() http.Header
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

func (r *HttpError) Headers() http.Header {
	return nil
}

func (r *HttpError) Error() string {
	return r.Body
}

func (r *HttpError) GetStatusCode() int {
	return r.StatusCode
}

type RedirectError struct {
	URL *url.URL
}

func (r *RedirectError) Error() string {
	return fmt.Sprintf("Redirect(url=%v)", r.URL)
}

func (r *RedirectError) GetStatusCode() int {
	return http.StatusFound
}

func (r *RedirectError) Headers() http.Header {
	h := make(http.Header)
	h.Set("Location", r.URL.String())
	return h
}
