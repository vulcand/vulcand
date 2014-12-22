// Interfaces for request limiting
package limit

import (
	"fmt"
	"github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcan/request"
	"strings"
)

// Limiter is an interface for request limiters (e.g. rate/connection) limiters
type Limiter interface {
	// In case if limiter wants to reject request, it should return http response
	// will be proxied to the client.
	// In case if limiter returns an error, it will be treated as a request error and will
	// potentially activate failure recovery and failover algorithms.
	// In case if lmimiter wants to delay request, it should return duration > 0
	// Otherwise limiter should return (0, nil) to allow request to proceed
	middleware.Middleware
}

// MapperFn takes the request and returns token that corresponds to the  request and the amount of tokens this request is going to consume, e.g.
// * Client ip rate limiter - token is a client ip, amount is 1 request
// * Client ip bandwidth limiter - token is a client ip, amount is number of bytes to consume
// In case of error returns non nil error, in this case rate limiter will reject the request.
type MapperFn func(r request.Request) (token string, amount int64, err error)

// TokenMapperFn maps the request to limiting token
type TokenMapperFn func(r request.Request) (token string, err error)

// AmountMapperFn maps the request to the amount of tokens to consume
type AmountMapperFn func(r request.Request) (amount int64, err error)

// MapClientIp creates a mapper that allows rate limiting of requests per client ip
func MapClientIp(req request.Request) (string, int64, error) {
	t, err := RequestToClientIp(req)
	return t, 1, err
}

func MapRequestHost(req request.Request) (string, int64, error) {
	t, err := RequestToHost(req)
	return t, 1, err
}

func MakeMapRequestHeader(header string) MapperFn {
	return MakeMapper(MakeRequestToHeader(header), RequestToCount)
}

func VariableToMapper(variable string) (MapperFn, error) {
	tokenMapper, err := MakeTokenMapperFromVariable(variable)
	if err != nil {
		return nil, err
	}
	return MakeMapper(tokenMapper, RequestToCount), nil
}

// Make mapper constructs the mapper function out of two functions - token mapper and amount mapper
func MakeMapper(t TokenMapperFn, a AmountMapperFn) MapperFn {
	return func(r request.Request) (string, int64, error) {
		token, err := t(r)
		if err != nil {
			return "", -1, err
		}
		amount, err := a(r)
		if err != nil {
			return "", -1, err
		}
		return token, amount, nil
	}
}

// RequestToClientIp is a TokenMapper that maps the request to the client IP.
func RequestToClientIp(req request.Request) (string, error) {
	vals := strings.SplitN(req.GetHttpRequest().RemoteAddr, ":", 2)
	if len(vals[0]) == 0 {
		return "", fmt.Errorf("Failed to parse client IP")
	}
	return vals[0], nil
}

// RequestToHost maps request to the host value
func RequestToHost(req request.Request) (string, error) {
	return req.GetHttpRequest().Host, nil
}

// RequestToCount maps request to the amount of requests (essentially one)
func RequestToCount(req request.Request) (int64, error) {
	return 1, nil
}

// Maps request to it's size in bytes
func RequestToBytes(req request.Request) (int64, error) {
	return req.GetBody().TotalSize()
}

// MakeTokenMapperByHeader creates a TokenMapper that maps the incoming request to the header value.
func MakeRequestToHeader(header string) TokenMapperFn {
	return func(req request.Request) (string, error) {
		return req.GetHttpRequest().Header.Get(header), nil
	}
}

// Converts varaiable string to a mapper function used in limiters
func MakeTokenMapperFromVariable(variable string) (TokenMapperFn, error) {
	if variable == "client.ip" {
		return RequestToClientIp, nil
	}
	if variable == "request.host" {
		return RequestToHost, nil
	}
	if strings.HasPrefix(variable, "request.header.") {
		header := strings.TrimPrefix(variable, "request.header.")
		if len(header) == 0 {
			return nil, fmt.Errorf("Wrong header: %s", header)
		}
		return MakeRequestToHeader(header), nil
	}
	return nil, fmt.Errorf("Unsupported limiting variable: '%s'", variable)
}
