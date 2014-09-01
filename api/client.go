package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/plugin"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const CurrentVersion = "v1"

type Client struct {
	Addr     string
	Registry *Registry
}

func NewClient(addr string, registry *Registry) *Client {
	return &Client{Addr: addr, Registry: registry}
}

func (c *Client) GetStatus() error {
	_, err := c.Get(c.endpoint("status"), url.Values{})
	return err
}

func (c *Client) GetHosts() ([]*Host, error) {
	data, err := c.Get(c.endpoint("hosts"), url.Values{})
	if err != nil {
		return nil, err
	}
	return HostsFromJson(data, c.Registry.GetSpec)
}

func (c *Client) AddHost(name string) (*Host, error) {
	host, err := NewHost(name)
	if err != nil {
		return nil, err
	}
	response, err := c.Post(c.endpoint("hosts"), host)
	if err != nil {
		return nil, err
	}
	return HostFromJson(response, c.Registry.GetSpec)
}

func (c *Client) UpdateHostCert(hostname string, cert *Certificate) (*Host, error) {
	response, err := c.Put(c.endpoint("hosts", hostname, "cert"), cert)
	if err != nil {
		return nil, err
	}
	return HostFromJson(response, c.Registry.GetSpec)
}

func (c *Client) DeleteHost(name string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("hosts", name))
}

func (c *Client) AddLocation(hostname, id, path, upstream string) (*Location, error) {
	return c.AddLocationWithOptions(hostname, id, path, upstream, LocationOptions{})
}

func (c *Client) AddLocationWithOptions(hostname, id, path, upstream string, options LocationOptions) (*Location, error) {
	location, err := NewLocationWithOptions(hostname, id, path, upstream, options)
	if err != nil {
		return nil, err
	}
	response, err := c.Post(c.endpoint("hosts", hostname, "locations"), location)
	if err != nil {
		return nil, err
	}
	return LocationFromJson(response, c.Registry.GetSpec)
}

func (c *Client) DeleteLocation(hostname, id string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("hosts", hostname, "locations", id))
}

func (c *Client) UpdateLocationUpstream(hostname, location, upstream string) (*StatusResponse, error) {
	return c.PutForm(c.endpoint("hosts", hostname, "locations", location), url.Values{"upstream": {upstream}})
}

func (c *Client) UpdateLocationOptions(hostname, location string, options LocationOptions) (*Location, error) {
	response, err := c.Put(c.endpoint("hosts", hostname, "locations", location, "options"), options)
	if err != nil {
		return nil, err
	}
	return LocationFromJson(response, c.Registry.GetSpec)
}

func (c *Client) AddUpstream(id string) (*Upstream, error) {
	upstream, err := NewUpstream(id)
	if err != nil {
		return nil, err
	}
	response, err := c.Post(c.endpoint("upstreams"), upstream)
	if err != nil {
		return nil, err
	}
	return UpstreamFromJson(response)
}

func (c *Client) DeleteUpstream(id string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("upstreams", id))
}

func (c *Client) GetUpstream(id string) (*Upstream, error) {
	response, err := c.Get(c.endpoint("upstreams", id), url.Values{})
	if err != nil {
		return nil, err
	}
	return UpstreamFromJson(response)
}

func (c *Client) DrainUpstreamConnections(upstreamId, timeout string) (int, error) {
	data, err := c.Get(c.endpoint("upstreams", upstreamId, "drain"), url.Values{"timeout": {timeout}})
	if err != nil {
		return -1, err
	}
	var connections *ConnectionsResponse
	if err := json.Unmarshal(data, &connections); err != nil {
		return -1, err
	}
	return connections.Connections, nil
}

func (c *Client) GetUpstreams() ([]*Upstream, error) {
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

func (c *Client) AddEndpoint(upstreamId, id, u string) (*Endpoint, error) {
	e, err := NewEndpoint(upstreamId, id, u)
	if err != nil {
		return nil, err
	}
	data, err := c.Post(c.endpoint("upstreams", upstreamId, "endpoints"), e)
	if err != nil {
		return nil, err
	}
	return EndpointFromJson(data)
}

func (c *Client) DeleteEndpoint(upstreamId, id string) (*StatusResponse, error) {
	return c.Delete(c.endpoint("upstreams", upstreamId, "endpoints", id))
}

func (c *Client) AddMiddleware(spec *MiddlewareSpec, hostname, locationId string, m *MiddlewareInstance) (*MiddlewareInstance, error) {
	data, err := c.Post(
		c.endpoint("hosts", hostname, "locations", locationId, "middlewares", spec.Type), m)
	if err != nil {
		return nil, err
	}
	return MiddlewareFromJson(data, c.Registry.GetSpec)
}

func (c *Client) UpdateMiddleware(spec *MiddlewareSpec, hostname, locationId string, m *MiddlewareInstance) (*MiddlewareInstance, error) {
	data, err := c.Put(
		c.endpoint("hosts", hostname, "locations", locationId, "middlewares", spec.Type, m.Id), m)
	if err != nil {
		return nil, err
	}
	return MiddlewareFromJson(data, c.Registry.GetSpec)
}

func (c *Client) DeleteMiddleware(spec *MiddlewareSpec, hostname, locationId, mId string) (*StatusResponse, error) {
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

func (c *Client) PostForm(endpoint string, values url.Values) ([]byte, error) {
	return c.RoundTrip(func() (*http.Response, error) {
		return http.PostForm(endpoint, values)
	})
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
			return nil, fmt.Errorf("Failed to decode response '%s', error: %", responseBody, err)
		}
		if response.StatusCode == http.StatusNotFound {
			return nil, &NotFoundError{Message: status.Message}
		}
		if response.StatusCode == http.StatusConflict {
			return nil, &AlreadyExistsError{Message: status.Message}
		}
		return nil, status
	}
	return responseBody, nil
}

func (c *Client) endpoint(params ...string) string {
	return fmt.Sprintf("%s/%s/%s", c.Addr, CurrentVersion, strings.Join(params, "/"))
}

type HostsResponse struct {
	Hosts []*Host
}

type UpstreamsResponse struct {
	Upstreams []*Upstream
}

type UpstreamResponse struct {
	Upstream *Upstream
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
