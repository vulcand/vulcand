/*Endpoints - final destination of the http request
 */
package endpoint

import (
	"fmt"
	"github.com/mailgun/vulcan/netutils"
	"net/url"
)

type Endpoint interface {
	GetId() string
	GetUrl() *url.URL
	String() string
}

type HttpEndpoint struct {
	url *url.URL
	id  string
}

func ParseUrl(in string) (*HttpEndpoint, error) {
	url, err := netutils.ParseUrl(in)
	if err != nil {
		return nil, err
	}
	return &HttpEndpoint{url: url, id: fmt.Sprintf("%s://%s", url.Scheme, url.Host)}, nil
}

func MustParseUrl(in string) *HttpEndpoint {
	u, err := ParseUrl(in)
	if err != nil {
		panic(err)
	}
	return u
}

func NewHttpEndpoint(in *url.URL) (*HttpEndpoint, error) {
	if in == nil {
		return nil, fmt.Errorf("Provide url")
	}
	return &HttpEndpoint{
		url: netutils.CopyUrl(in),
		id:  fmt.Sprintf("%s://%s", in.Scheme, in.Host)}, nil
}

func (e *HttpEndpoint) String() string {
	return e.url.String()
}

func (e *HttpEndpoint) GetId() string {
	return e.id
}

func (e *HttpEndpoint) GetUrl() *url.URL {
	return e.url
}
