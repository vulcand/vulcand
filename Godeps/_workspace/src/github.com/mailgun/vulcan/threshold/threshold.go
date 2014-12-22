/*
Package threshold contains predicates that can define various request thresholds

Examples:

* RequestMethod() == "GET" triggers action when request method equals "GET"
* IsNetworkError() - triggers action on network errors
* RequestMethod() == "GET" && Attempts <= 2 && (IsNetworkError() || ResponseCode() == 408)
  This predicate triggers for GET requests with maximum 2 attempts
  on network errors or when upstream returns special http response code 408
*/
package threshold

import (
	"fmt"

	"github.com/mailgun/vulcan/request"
)

// Predicate that defines what request can fail over in case of error or http response
type Predicate func(request.Request) bool

// RequestToString defines mapper function that maps a request to some string (e.g extracts method name)
type RequestToString func(req request.Request) string

// RequestToInt defines mapper function that maps a request to some int (e.g extracts response code)
type RequestToInt func(req request.Request) int

// RequestToFloat64 defines mapper function that maps a request to some float64 (e.g extracts some ratio)
type RequestToFloat64 func(req request.Request) float64

// RequestMethod returns mapper of the request to its method e.g. POST
func RequestMethod() RequestToString {
	return func(r request.Request) string {
		return r.GetHttpRequest().Method
	}
}

// Attempts returns mapper of the request to the number of proxy attempts
func Attempts() RequestToInt {
	return func(r request.Request) int {
		return len(r.GetAttempts())
	}
}

// ResponseCode returns mapper of the request to the last response code, returns 0 if there was no response code.
func ResponseCode() RequestToInt {
	return func(r request.Request) int {
		attempts := len(r.GetAttempts())
		if attempts == 0 {
			return 0
		}
		lastResponse := r.GetAttempts()[attempts-1].GetResponse()
		if lastResponse == nil {
			return 0
		}
		return lastResponse.StatusCode
	}
}

// IsNetworkError returns a predicate that returns true if last attempt ended with network error.
func IsNetworkError() Predicate {
	return func(r request.Request) bool {
		attempts := len(r.GetAttempts())
		return attempts != 0 && r.GetAttempts()[attempts-1].GetError() != nil
	}
}

// AND returns predicate by joining the passed predicates with logical 'and'
func AND(fns ...Predicate) Predicate {
	return func(req request.Request) bool {
		for _, fn := range fns {
			if !fn(req) {
				return false
			}
		}
		return true
	}
}

// OR returns predicate by joining the passed predicates with logical 'or'
func OR(fns ...Predicate) Predicate {
	return func(req request.Request) bool {
		for _, fn := range fns {
			if fn(req) {
				return true
			}
		}
		return false
	}
}

// NOT creates negation of the passed predicate
func NOT(p Predicate) Predicate {
	return func(r request.Request) bool {
		return !p(r)
	}
}

// EQ returns predicate that tests for equality of the value of the mapper and the constant
func EQ(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case RequestToString:
		return stringEQ(mapper, value)
	case RequestToInt:
		return intEQ(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

// NEQ returns predicate that tests for inequality of the value of the mapper and the constant
func NEQ(m interface{}, value interface{}) (Predicate, error) {
	p, err := EQ(m, value)
	if err != nil {
		return nil, err
	}
	return NOT(p), nil
}

// LT returns predicate that tests that value of the mapper function is less than the constant
func LT(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case RequestToInt:
		return intLT(mapper, value)
	case RequestToFloat64:
		return float64LT(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

// GT returns predicate that tests that value of the mapper function is greater than the constant
func GT(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case RequestToInt:
		return intGT(mapper, value)
	case RequestToFloat64:
		return float64GT(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

// LE returns predicate that tests that value of the mapper function is less than or equal to the constant
func LE(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case RequestToInt:
		return intLE(mapper, value)
	case RequestToFloat64:
		return float64LE(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

// GE returns predicate that tests that value of the mapper function is greater than or equal to the constant
func GE(m interface{}, value interface{}) (Predicate, error) {
	switch mapper := m.(type) {
	case RequestToInt:
		return intGE(mapper, value)
	case RequestToFloat64:
		return float64GE(mapper, value)
	}
	return nil, fmt.Errorf("unsupported argument: %T", m)
}

func stringEQ(m RequestToString, val interface{}) (Predicate, error) {
	value, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("expected string, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) == value
	}, nil
}

func intEQ(m RequestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) == value
	}, nil
}

func intLT(m RequestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) < value
	}, nil
}

func intGT(m RequestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) > value
	}, nil
}

func intLE(m RequestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) <= value
	}, nil
}

func intGE(m RequestToInt, val interface{}) (Predicate, error) {
	value, ok := val.(int)
	if !ok {
		return nil, fmt.Errorf("expected int, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) >= value
	}, nil
}

func float64EQ(m RequestToFloat64, val interface{}) (Predicate, error) {
	value, ok := val.(float64)
	if !ok {
		return nil, fmt.Errorf("expected float64, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) == value
	}, nil
}

func float64LT(m RequestToFloat64, val interface{}) (Predicate, error) {
	value, ok := val.(float64)
	if !ok {
		return nil, fmt.Errorf("expected float64, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) < value
	}, nil
}

func float64GT(m RequestToFloat64, val interface{}) (Predicate, error) {
	value, ok := val.(float64)
	if !ok {
		return nil, fmt.Errorf("expected float64, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) > value
	}, nil
}

func float64LE(m RequestToFloat64, val interface{}) (Predicate, error) {
	value, ok := val.(float64)
	if !ok {
		return nil, fmt.Errorf("expected float64, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) <= value
	}, nil
}

func float64GE(m RequestToFloat64, val interface{}) (Predicate, error) {
	value, ok := val.(float64)
	if !ok {
		return nil, fmt.Errorf("expected float64, got %T", val)
	}
	return func(req request.Request) bool {
		return m(req) >= value
	}, nil
}
