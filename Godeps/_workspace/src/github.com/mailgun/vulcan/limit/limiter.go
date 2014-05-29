// Interfaces for request limiting
package limit

import (
	"fmt"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
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
	Middleware
}

// Mapper function takes the request and returns token that corresponds to this request
// and the amount of tokens this request is going to consume, e.g.
// * Client ip rate limiter - token is a client ip, amount is 1 request
// * Client ip memory limiter - token is a client ip, amount is number of bytes to consume
// In case of error returns non nil error, in this case rate limiter will reject the request.
type MapperFn func(r Request) (token string, amount int, err error)

// This function maps the request to it's client ip. Rate limiter using this mapper
// function will do rate limiting based on the client ip.
func MapClientIp(req Request) (string, int, error) {
	vals := strings.SplitN(req.GetHttpRequest().RemoteAddr, ":", 2)
	if len(vals[0]) == 0 {
		return "", -1, fmt.Errorf("Failed to parse client ip")
	}
	return vals[0], 1, nil
}

func MapRequestHost(req Request) (string, int, error) {
	return req.GetHttpRequest().Host, 1, nil
}

func MakeMapRequestHeader(header string) MapperFn {
	return func(req Request) (string, int, error) {
		return req.GetHttpRequest().Header.Get(header), 1, nil
	}
}

// Converts varaiable string to a mapper function used in rate limiters
func VariableToMapper(variable string) (MapperFn, error) {
	if variable == "client.ip" {
		return MapClientIp, nil
	}
	if variable == "request.host" {
		return MapRequestHost, nil
	}
	if strings.HasPrefix(variable, "request.header.") {
		header := strings.TrimPrefix(variable, "request.header.")
		if len(header) == 0 {
			return nil, fmt.Errorf("Wrong header: %s", header)
		}
		return MakeMapRequestHeader(header), nil
	}
	return nil, fmt.Errorf("Unsupported limiting variable: '%s'", variable)
}
