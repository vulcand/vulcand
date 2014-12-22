// Interfaces for location - round trip the http request to backends
package location

import (
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/request"
	"net/http"
)

// Location accepts proxy request and round trips it to the backend
type Location interface {
	// Unique identifier of this location
	GetId() string
	// Forward the request to a specific location and return the response
	RoundTrip(request.Request) (*http.Response, error)
}

// This location is used in tests
type Loc struct {
	Id   string
	Name string
}

func (*Loc) RoundTrip(request.Request) (*http.Response, error) {
	return nil, nil
}

func (l *Loc) GetId() string {
	return l.Id
}

// The simplest HTTP location implementation that adds no additional logic
// on top of simple http round trip function call
type ConstHttpLocation struct {
	Url string
}

func (l *ConstHttpLocation) RoundTrip(r request.Request) (*http.Response, error) {
	req := r.GetHttpRequest()
	req.URL = netutils.MustParseUrl(l.Url)
	return http.DefaultTransport.RoundTrip(req)
}

func (l *ConstHttpLocation) GetId() string {
	return l.Url
}
