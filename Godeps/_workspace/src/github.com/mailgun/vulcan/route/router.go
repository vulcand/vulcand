// Route the request to a location
package route

import (
	. "github.com/mailgun/vulcan/location"
	. "github.com/mailgun/vulcan/request"
)

// Router matches incoming request to a specific location
type Router interface {
	// if error is not nil, the request wll be aborted and error will be proxied to client.
	// if location is nil and error is nil, that means that router did not find any matching location
	Route(req Request) (Location, error)
}

// Helper router that always the same location
type ConstRouter struct {
	Location Location
}

func (m *ConstRouter) Route(req Request) (Location, error) {
	return m.Location, nil
}
