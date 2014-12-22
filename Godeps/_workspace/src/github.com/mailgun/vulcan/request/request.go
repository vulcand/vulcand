// Wrapper around http.Request with additional features
package request

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/netutils"
)

// Request is a rapper around http request that provides more info about http.Request
type Request interface {
	GetHttpRequest() *http.Request              // Original http request
	SetHttpRequest(*http.Request)               // Can be used to set http request
	GetId() int64                               // Request id that is unique to this running process
	SetBody(netutils.MultiReader)               // Sets request body
	GetBody() netutils.MultiReader              // Request body fully read and stored in effective manner (buffered to disk for large requests)
	AddAttempt(Attempt)                         // Add last proxy attempt to the request
	GetAttempts() []Attempt                     // Returns last attempts to proxy request, may be nil if there are no attempts
	GetLastAttempt() Attempt                    // Convenience method returning the last attempt, may be nil if there are no attempts
	String() string                             // Debugging string representation of the request
	SetUserData(key string, baton interface{})  // Provide storage space for data that survives with the request
	GetUserData(key string) (interface{}, bool) // Fetch user data set from previously SetUserData call
	DeleteUserData(key string)                  // Clean up user data set from previously SetUserData call
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
	HttpRequest   *http.Request
	Id            int64
	Body          netutils.MultiReader
	Attempts      []Attempt
	userDataMutex *sync.RWMutex
	userData      map[string]interface{}
}

func NewBaseRequest(r *http.Request, id int64, body netutils.MultiReader) *BaseRequest {
	return &BaseRequest{
		HttpRequest:   r,
		Id:            id,
		Body:          body,
		userDataMutex: &sync.RWMutex{},
	}

}

func (br *BaseRequest) String() string {
	return fmt.Sprintf("Request(id=%d, method=%s, url=%s, attempts=%d)", br.Id, br.HttpRequest.Method, br.HttpRequest.URL.String(), len(br.Attempts))
}

func (br *BaseRequest) GetHttpRequest() *http.Request {
	return br.HttpRequest
}

func (br *BaseRequest) SetHttpRequest(r *http.Request) {
	br.HttpRequest = r
}

func (br *BaseRequest) GetId() int64 {
	return br.Id
}

func (br *BaseRequest) SetBody(b netutils.MultiReader) {
	br.Body = b
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
func (br *BaseRequest) SetUserData(key string, baton interface{}) {
	br.userDataMutex.Lock()
	defer br.userDataMutex.Unlock()
	if br.userData == nil {
		br.userData = make(map[string]interface{})
	}
	br.userData[key] = baton
}
func (br *BaseRequest) GetUserData(key string) (i interface{}, b bool) {
	br.userDataMutex.RLock()
	defer br.userDataMutex.RUnlock()
	if br.userData == nil {
		return i, false
	}
	i, b = br.userData[key]
	return i, b
}
func (br *BaseRequest) DeleteUserData(key string) {
	br.userDataMutex.Lock()
	defer br.userDataMutex.Unlock()
	if br.userData == nil {
		return
	}

	delete(br.userData, key)
}
