package testutils

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin/ratelimit"
)

func GETResponse(c *C, url string, host string) string {
	response, body, err := GET(url, host)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	return string(body)
}

func GET(url string, host string) (*http.Response, []byte, error) {
	request, _ := http.NewRequest("GET", url, strings.NewReader(""))
	if len(host) != 0 {
		request.Host = host
	}

	var tr *http.Transport
	if strings.HasPrefix(url, "https") {
		tr = &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		}
	} else {
		tr = &http.Transport{
			DisableKeepAlives: true,
		}
	}

	client := &http.Client{Transport: tr}
	response, err := client.Do(request)
	if err == nil {
		bodyBytes, err := ioutil.ReadAll(response.Body)
		return response, bodyBytes, err
	}
	return response, nil, err
}

func NewTestServer(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	}))
}

func MakeLocation(hostname, listenerAddress, endpointURL string) (*Location, *Host) {
	host := &Host{
		Name: hostname,
		Listeners: []*Listener{
			&Listener{Protocol: HTTP, Address: Address{"tcp", listenerAddress}}},
	}

	upstream := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				UpstreamId: "up1",
				Id:         endpointURL,
				Url:        endpointURL,
			},
		},
	}
	location := &Location{
		Hostname: host.Name,
		Path:     "/loc1",
		Id:       "loc1",
		Upstream: upstream,
	}
	return location, host
}

func MakeRateLimit(id string, rate int, variable string, burst int64, periodSeconds int, loc *Location) *MiddlewareInstance {
	rl, err := ratelimit.NewRateLimit(rate, variable, burst, periodSeconds)
	if err != nil {
		panic(err)
	}
	return &MiddlewareInstance{
		Type:       "ratelimit",
		Id:         id,
		Middleware: rl,
	}
}

func MakeURL(loc *Location, l *Listener) string {
	return fmt.Sprintf("%s://%s%s", l.Protocol, l.Address.Address, loc.Path)
}

func AssertSameEndpoints(c *C, a []*roundrobin.WeightedEndpoint, b []*Endpoint) {
	x, y := map[string]bool{}, map[string]bool{}
	for _, e := range a {
		x[e.GetUrl().String()] = true
	}

	for _, e := range b {
		y[e.Url] = true
	}
	c.Assert(x, DeepEquals, y)
}
