// Predicates controlling when request should be retried
package failover

/*
Examples:

* OnGets - allows to failover only get requests
* OnErrors - allows to failover on errors

Example:

And(OnGets, MaxFails(2), Or(OnErrors, OnResponseCode(408))

This will create predicate that allows failover for get requests with maximum 2 proxy attempts with failover
triggered on errors or when upstream returns special http response code 408.
*/

import (
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

// Predicate that defines what request can fail over in case of error or http response
type Predicate func(Request) bool

// Predicate that allows failover for GET requests that errored
func OnGets(req Request) bool {
	return req.GetHttpRequest().Method == "GET"
}

// Failover in case if last attempt resulted in error
func OnErrors(req Request) bool {
	attempts := len(req.GetAttempts())
	return attempts != 0 && req.GetAttempts()[attempts-1].GetError() != nil
}

// Function that returns predicate by joining the passed predicates with AND
func And(fns ...Predicate) Predicate {
	return func(req Request) bool {
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
	return func(req Request) bool {
		for _, fn := range fns {
			if fn(req) {
				return true
			}
		}
		return false
	}
}

// Function that returns predicate allowing certain number of attempts
func MaxAttempts(count int) Predicate {
	return func(req Request) bool {
		return len(req.GetAttempts()) <= count
	}
}

// Function that returns predicate triggering failover in case if proxy returned certain http code
func ResponseCode(code int) Predicate {
	return func(req Request) bool {
		attempts := len(req.GetAttempts())
		if attempts == 0 {
			return false
		}
		lastResponse := req.GetAttempts()[attempts-1].GetResponse()
		return lastResponse != nil && lastResponse.StatusCode == code
	}
}
