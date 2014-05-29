// Wrapper around http.Request with additional features
package request

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"net/http"
	"time"
)

// Wrapper around http request that provides more info about http.Request
type Request interface {
	GetHttpRequest() *http.Request // Original http request
	GetId() int64                  // Request id that is unique to this running process
	GetBody() netutils.MultiReader // Request body fully read and stored in effective manner (buffered to disk for large requests)
	AddAttempt(Attempt)            // Add last proxy attempt to the request
	GetAttempts() []Attempt        // Returns last attempts to proxy request, may be nil if there are no attempts
	GetLastAttempt() Attempt       // Convenience method returning the last attempt, may be nil if there are no attempts
	String() string                // Debugging string representation of the request
}

type Attempt interface {
	GetError() error
	GetDuration() time.Duration
	GetResponse() *http.Response
	GetEndpoint() endpoint.Endpoint
}

type BaseAttempt struct {
	Error    error
	Duration time.Duration
	Response *http.Response
	Endpoint endpoint.Endpoint
}

func (ba *BaseAttempt) GetResponse() *http.Response {
	return ba.Response
}

func (ba *BaseAttempt) GetError() error {
	return ba.Error
}

func (ba *BaseAttempt) GetDuration() time.Duration {
	return ba.Duration
}

func (ba *BaseAttempt) GetEndpoint() endpoint.Endpoint {
	return ba.Endpoint
}

type BaseRequest struct {
	HttpRequest *http.Request
	Id          int64
	Body        netutils.MultiReader
	Attempts    []Attempt
}

func (br *BaseRequest) String() string {
	return fmt.Sprintf("Request(id=%d, method=%s, url=%s, attempts=%d)", br.Id, br.HttpRequest.Method, br.HttpRequest.URL.String(), len(br.Attempts))
}

func (br *BaseRequest) GetHttpRequest() *http.Request {
	return br.HttpRequest
}

func (br *BaseRequest) GetId() int64 {
	return br.Id
}

func (br *BaseRequest) GetBody() netutils.MultiReader {
	return br.Body
}

func (br *BaseRequest) AddAttempt(a Attempt) {
	br.Attempts = append(br.Attempts, a)
}

func (br *BaseRequest) GetAttempts() []Attempt {
	return br.Attempts
}

func (br *BaseRequest) GetLastAttempt() Attempt {
	if len(br.Attempts) == 0 {
		return nil
	}
	return br.Attempts[len(br.Attempts)-1]
}
