// Middlewares can modify or intercept requests and responses
package middleware

import (
	. "github.com/mailgun/vulcan/request"
	"net/http"
)

// Middlewares are allowed to observe, modify and intercept http requests and responses
type Middleware interface {
	// Called before the request is going to be proxied to the endpoint selected by the load balancer.
	// If it returns an error, request will be treated as erorrneous (e.g. failover will be initated).
	// If it returns a non nil response, proxy will return the response without proxying to the endpoint.
	// If it returns nil response and nil error request will be proxied to the upstream.
	// It's ok to modify request headers and body as a side effect of the funciton call.
	ProcessRequest(r Request) (*http.Response, error)

	// If request has been completed or intercepted by middleware and response has been received
	// attempt would contain non nil response or non nil error.
	ProcessResponse(r Request, a Attempt)
}

// Unlinke middlewares, observers are not able to intercept or change any requests
// and will be called on every request to endpoint regardless of the middlewares side effects
type Observer interface {
	// Will be called before every request to the endpoint
	ObserveRequest(r Request)

	// Will be called after every request to the endpoint
	ObserveResponse(r Request, a Attempt)
}

type ProcessRequestFn func(r Request) (*http.Response, error)
type ProcessResponseFn func(r Request, a Attempt)

// Wraps the functions to create a middleware compatible interface
type MiddlewareWrapper struct {
	OnRequest  ProcessRequestFn
	OnResponse ProcessResponseFn
}

func (cb *MiddlewareWrapper) ProcessRequest(r Request) (*http.Response, error) {
	if cb.OnRequest != nil {
		return cb.OnRequest(r)
	}
	return nil, nil
}

func (cb *MiddlewareWrapper) ProcessResponse(r Request, a Attempt) {
	if cb.OnResponse != nil {
		cb.OnResponse(r, a)
	}
}

type ObserveRequestFn func(r Request)
type ObserveResponseFn func(r Request, a Attempt)

// Wraps the functions to create a observer compatible interface
type ObserverWrapper struct {
	OnRequest  ObserveRequestFn
	OnResponse ObserveResponseFn
}

func (cb *ObserverWrapper) ObserveRequest(r Request) {
	if cb.OnRequest != nil {
		cb.OnRequest(r)
	}
}

func (cb *ObserverWrapper) ObserveResponse(r Request, a Attempt) {
	if cb.OnResponse != nil {
		cb.OnResponse(r, a)
	}
}
