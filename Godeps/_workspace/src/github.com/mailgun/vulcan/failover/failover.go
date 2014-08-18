// Package failover contains predicates that define when request should be retried.
package failover

/*
Examples:

* RequestMethodEq("GET") - allows to failover only get requests
* IsNetworkError - allows to failover on errors
* RequestMethodEq("GET") && AttemptsLe(2) && (IsNetworkError || ResponseCodeEq(408))
  This predicate allows failover for GET requests with maximum 2 attempts with failover
  triggered on network errors or when upstream returns special http response code 408.
*/

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

// Predicate that defines what request can fail over in case of error or http response
type Predicate func(request.Request) bool

func RequestMethodEq(method string) Predicate {
	return func(req request.Request) bool {
		return req.GetHttpRequest().Method == method
	}
}

// Failover in case if last attempt resulted in error
func IsNetworkError(req request.Request) bool {
	attempts := len(req.GetAttempts())
	return attempts != 0 && req.GetAttempts()[attempts-1].GetError() != nil
}

// Function that returns predicate by joining the passed predicates with AND
func And(fns ...Predicate) Predicate {
	return func(req request.Request) bool {
		for _, fn := range fns {
			if !fn(req) {
				return false
			}
		}
		return true
	}
}

// Function that returns predicate by joining the passed predicates with OR
func Or(fns ...Predicate) Predicate {
	return func(req request.Request) bool {
		for _, fn := range fns {
			if fn(req) {
				return true
			}
		}
		return false
	}
}

// Function that returns predicate allowing certain number of attempts
func AttemptsLe(count int) Predicate {
	return func(req request.Request) bool {
		return len(req.GetAttempts()) <= count
	}
}

// Function that returns predicate triggering failover in case if proxy returned certain http code
func ResponseCodeEq(code int) Predicate {
	return func(req request.Request) bool {
		attempts := len(req.GetAttempts())
		if attempts == 0 {
			return false
		}
		lastResponse := req.GetAttempts()[attempts-1].GetResponse()
		return lastResponse != nil && lastResponse.StatusCode == code
	}
}
