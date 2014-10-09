package server

import (
	"fmt"
	"net/url"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/backend"
)

type muxEndpoint struct {
	url      *url.URL
	id       string
	location *backend.Location
	endpoint *backend.Endpoint
}

func newEndpoint(loc *backend.Location, e *backend.Endpoint) (*muxEndpoint, error) {
	url, err := netutils.ParseUrl(e.Url)
	if err != nil {
		return nil, err
	}
	return &muxEndpoint{location: loc, endpoint: e, id: e.GetUniqueId().String(), url: url}, nil
}

func (e *muxEndpoint) String() string {
	return fmt.Sprintf("MuxEndpoint(id=%s, url=%s)", e.id, e.url.String())
}

func (e *muxEndpoint) GetId() string {
	return e.id
}

func (e *muxEndpoint) GetUrl() *url.URL {
	return e.url
}
