package api

import (
	"bytes"
	"encoding/json"
	"fmt"

	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
)

const CurrentVersion = "v1"

type Client struct {
	Addr     string
	Registry *plugin.Registry
}

func NewClient(addr string, registry *plugin.Registry) *Client {
	return &Client{Addr: addr, Registry: registry}
}

func (c *Client) GetStatus() error {
	_, err := c.Get(c.endpoint("status"), url.Values{})
	return err
}

func (c *Client) GetHosts() ([]*backend.Host, error) {
	data, err := c.Get(c.endpoint("hosts"), url.Values{})
	if err != nil {
		return nil, err
	}
	return backend.HostsFromJSON(data, c.Registry.GetSpec)
}

func (c *Client) UpdateLogSeverity(s log.Severity) (*StatusResponse, error) {
	return c.PutForm(c.endpoint("log", "severity"), url.Values{"severity": {s.String()}})
}

func (c *Client) GetLogSeverity() (log.Severity, error) {
	data, err := c.Get(c.endpoint("log", "severity"), url.Values{})
	if err != nil {
		return -1, err
	}
	var sev *SeverityResponse
	if err := json.Unmarshal(data, &sev); err != nil {
		return -1, err
	}
	return log.SeverityFromString(sev.Severity)
}

func (c *Client) GetHost(name string) (*backend.Host, error) {
	response, err := c.Get(c.endpoint("hosts", name), url.Values{})
	if err != nil {
		return nil, err
	}
	return backend.HostFromJSON(response, c.Registry.GetSpec)
}

func (c *Client) AddHost(name string) (*backend.Host, error) {
	host, err := backend.NewHost(name)
	if err != nil {
		return nil, err
	}
	response, err := c.Post(c.endpoint("hosts"), host)
	if err != nil {
		return nil, err
	}
	return backend.HostFromJSON(response, c.Registry.GetSpec)
}

func (c *Client) AddHostListener(hostname string, l *backend.Listener) (*backend.Listener, error) {
	response, err := c.Post(c.endpoint("hosts", hostname, "listeners"), l)
	if err != nil {
		return nil, err
	}
	return backend.ListenerFromJSON(response)
}

func (c *Client) DeleteHostListener(name, listenerId string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("hosts", name, "listeners", listenerId))
}

func (c *Client) UpdateHostKeyPair(hostname string, keyPair *backend.KeyPair) (*backend.Host, error) {
	response, err := c.Put(c.endpoint("hosts", hostname, "keypair"), keyPair)
	if err != nil {
		return nil, err
	}
	return backend.HostFromJSON(response, c.Registry.GetSpec)
}

func (c *Client) DeleteHost(name string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("hosts", name))
}

func (c *Client) AddLocation(hostname, id, path, upstream string) (*backend.Location, error) {
	return c.AddLocationWithOptions(hostname, id, path, upstream, backend.LocationOptions{})
}

func (c *Client) GetLocation(name, id string) (*backend.Location, error) {
	response, err := c.Get(c.endpoint("hosts", name, "locations", id), url.Values{})
	if err != nil {
		return nil, err
	}
	return backend.LocationFromJSON(response, c.Registry.GetSpec)
}

func (c *Client) GetTopLocations(hostname, upstreamId string, limit int) ([]*backend.Location, error) {
	response, err := c.Get(c.endpoint("hosts", "top", "locations"),
		url.Values{
			"hostname":   {hostname},
			"upstreamId": {upstreamId},
			"limit":      {fmt.Sprintf("%d", limit)},
		})
	if err != nil {
		return nil, err
	}
	return backend.LocationsFromJSON(response, c.Registry.GetSpec)
}

func (c *Client) AddLocationWithOptions(hostname, id, path, upstream string, options backend.LocationOptions) (*backend.Location, error) {
	location, err := backend.NewLocationWithOptions(hostname, id, path, upstream, options)
	if err != nil {
		return nil, err
	}
	response, err := c.Post(c.endpoint("hosts", hostname, "locations"), location)
	if err != nil {
		return nil, err
	}
	return backend.LocationFromJSON(response, c.Registry.GetSpec)
}

func (c *Client) DeleteLocation(hostname, id string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("hosts", hostname, "locations", id))
}

func (c *Client) UpdateLocationUpstream(hostname, location, upstream string) (*StatusResponse, error) {
	return c.PutForm(c.endpoint("hosts", hostname, "locations", location), url.Values{"upstream": {upstream}})
}

func (c *Client) UpdateLocationOptions(hostname, location string, options backend.LocationOptions) (*backend.Location, error) {
	response, err := c.Put(c.endpoint("hosts", hostname, "locations", location, "options"), options)
	if err != nil {
		return nil, err
	}
	return backend.LocationFromJSON(response, c.Registry.GetSpec)
}

func (c *Client) AddUpstreamWithOptions(id string, o backend.UpstreamOptions) (*backend.Upstream, error) {
	upstream, err := backend.NewUpstreamWithOptions(id, o)
	if err != nil {
		return nil, err
	}
	response, err := c.Post(c.endpoint("upstreams"), upstream)
	if err != nil {
		return nil, err
	}
	return backend.UpstreamFromJSON(response)
}

func (c *Client) UpdateUpstreamOptions(upId string, options backend.UpstreamOptions) (*backend.Upstream, error) {
	response, err := c.Put(c.endpoint("upstreams", upId, "options"), options)
	if err != nil {
		return nil, err
	}
	return backend.UpstreamFromJSON(response)
}

func (c *Client) AddUpstream(id string) (*backend.Upstream, error) {
	return c.AddUpstreamWithOptions(id, backend.UpstreamOptions{})
}

func (c *Client) DeleteUpstream(id string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("upstreams", id))
}

func (c *Client) GetUpstream(id string) (*backend.Upstream, error) {
	response, err := c.Get(c.endpoint("upstreams", id), url.Values{})
	if err != nil {
		return nil, err
	}
	return backend.UpstreamFromJSON(response)
}

func (c *Client) GetUpstreams() ([]*backend.Upstream, error) {
	data, err := c.Get(c.endpoint("upstreams"), url.Values{})
	if err != nil {
		return nil, err
	}
	var upstreams *UpstreamsResponse
	if err := json.Unmarshal(data, &upstreams); err != nil {
		return nil, err
	}
	return upstreams.Upstreams, nil
}

func (c *Client) AddEndpoint(upstreamId, id, u string) (*backend.Endpoint, error) {
	e, err := backend.NewEndpoint(upstreamId, id, u)
	if err != nil {
		return nil, err
	}
	data, err := c.Post(c.endpoint("upstreams", upstreamId, "endpoints"), e)
	if err != nil {
		return nil, err
	}
	return backend.EndpointFromJSON(data)
}

func (c *Client) GetTopEndpoints(upstreamId string, limit int) ([]*backend.Endpoint, error) {
	response, err := c.Get(c.endpoint("upstreams", "top", "endpoints"),
		url.Values{
			"upstreamId": {upstreamId},
			"limit":      {fmt.Sprintf("%d", limit)},
		})
	if err != nil {
		return nil, err
	}
	var re *EndpointsResponse
	if err = json.Unmarshal(response, &re); err != nil {
		return nil, err
	}
	return re.Endpoints, nil
}

func (c *Client) DeleteEndpoint(upstreamId, id string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("upstreams", upstreamId, "endpoints", id))
}

func (c *Client) AddMiddleware(spec *plugin.MiddlewareSpec, hostname, locationId string, m *backend.MiddlewareInstance) (*backend.MiddlewareInstance, error) {
	data, err := c.Post(
		c.endpoint("hosts", hostname, "locations", locationId, "middlewares", spec.Type), m)
	if err != nil {
		return nil, err
	}
	return backend.MiddlewareFromJSON(data, c.Registry.GetSpec)
}

func (c *Client) UpdateMiddleware(spec *plugin.MiddlewareSpec, hostname, locationId string, m *backend.MiddlewareInstance) (*backend.MiddlewareInstance, error) {
	data, err := c.Put(
		c.endpoint("hosts", hostname, "locations", locationId, "middlewares", spec.Type, m.Id), m)
	if err != nil {
		return nil, err
	}
	return backend.MiddlewareFromJSON(data, c.Registry.GetSpec)
}

func (c *Client) DeleteMiddleware(spec *plugin.MiddlewareSpec, hostname, locationId, mId string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("hosts", hostname, "locations", locationId, "middlewares", spec.Type, mId))
}

func (c *Client) PutForm(endpoint string, values url.Values) (*StatusResponse, error) {
	data, err := c.RoundTrip(func() (*http.Response, error) {
		req, err := http.NewRequest("PUT", endpoint, strings.NewReader(values.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return http.DefaultClient.Do(req)
	})
	if err != nil {
		return nil, err
	}
	var re *StatusResponse
	err = json.Unmarshal(data, &re)
	return re, err
}

func (c *Client) Post(endpoint string, in interface{}) ([]byte, error) {
	return c.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		return http.Post(endpoint, "application/json", bytes.NewBuffer(data))
	})
}

func (c *Client) Put(endpoint string, in interface{}) ([]byte, error) {
	return c.RoundTrip(func() (*http.Response, error) {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		re, err := http.DefaultClient.Do(req)
		return re, err
	})
}

func (c *Client) Delete(endpoint string) (*StatusResponse, error) {
	data, err := c.RoundTrip(func() (*http.Response, error) {
		req, err := http.NewRequest("DELETE", endpoint, nil)
		if err != nil {
			return nil, err
		}
		return http.DefaultClient.Do(req)
	})
	if err != nil {
		return nil, err
	}
	var re *StatusResponse
	err = json.Unmarshal(data, &re)
	return re, err
}

func (c *Client) Get(u string, params url.Values) ([]byte, error) {
	baseUrl, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	baseUrl.RawQuery = params.Encode()
	return c.RoundTrip(func() (*http.Response, error) {
		return http.Get(baseUrl.String())
	})
}

type RoundTripFn func() (*http.Response, error)

func (c *Client) RoundTrip(fn RoundTripFn) ([]byte, error) {
	response, err := fn()
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		var status *StatusResponse
		if err := json.Unmarshal(responseBody, &status); err != nil {
			return nil, fmt.Errorf("failed to decode response '%s', error: %v", responseBody, err)
		}
		if response.StatusCode == http.StatusNotFound {
			return nil, &backend.NotFoundError{Message: status.Message}
		}
		if response.StatusCode == http.StatusConflict {
			return nil, &backend.AlreadyExistsError{Message: status.Message}
		}
		return nil, status
	}
	return responseBody, nil
}

func (c *Client) endpoint(params ...string) string {
	return fmt.Sprintf("%s/%s/%s", c.Addr, CurrentVersion, strings.Join(params, "/"))
}

type UpstreamsResponse struct {
	Upstreams []*backend.Upstream
}

type EndpointsResponse struct {
	Endpoints []*backend.Endpoint
}

type StatusResponse struct {
	Message string
}

func (e *StatusResponse) Error() string {
	return e.Message
}

type ConnectionsResponse struct {
	Connections int
}

type SeverityResponse struct {
	Severity string
}
