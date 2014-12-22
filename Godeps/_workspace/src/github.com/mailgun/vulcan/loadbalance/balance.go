// Load balancers control how requests are distributed among multiple endpoints.
package loadbalance

import (
	. "github.com/mailgun/vulcan/endpoint"
	. "github.com/mailgun/vulcan/middleware"
	. "github.com/mailgun/vulcan/request"
)

type LoadBalancer interface {
	// This function will be called each time locaiton would need to choose the next endpoint for the request
	NextEndpoint(req Request) (Endpoint, error)
	// Load balancer can intercept the request
	Middleware
	// Load balancer may observe the request stats to get some runtime metrics
	Observer
}
