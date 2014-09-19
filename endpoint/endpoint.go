package endpoint

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"net/url"
)

type VulcanEndpoint struct {
	Url        *url.URL
	Id         string
	UpstreamId string
}

func EndpointFromUrl(upId, id, u string) (*VulcanEndpoint, error) {
	url, err := netutils.ParseUrl(u)
	if err != nil {
		return nil, err
	}
	return &VulcanEndpoint{Url: url, Id: id, UpstreamId: upId}, nil
}

func (e *VulcanEndpoint) String() string {
	return fmt.Sprintf("endpoint(id=%s, url=%s)", e.Id, e.Url.String())
}

func (e *VulcanEndpoint) GetId() string {
	return e.Id
}

func (e *VulcanEndpoint) GetUrl() *url.URL {
	return e.Url
}
