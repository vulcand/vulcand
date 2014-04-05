package main

import (
	"encoding/json"
	"fmt"
	. "github.com/mailgun/vulcand/backend"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const CurrentVersion = "v1"

type Client struct {
	Addr string
}

func NewClient(addr string) *Client {
	return &Client{Addr: addr}
}

func (c *Client) GetHosts() ([]*Host, error) {
	hosts := HostsResponse{}
	err := c.Get(c.endpoint("hosts"), &hosts)
	return hosts.Hosts, err
}

func (c *Client) AddHost(name string) error {
	response := StatusResponse{}
	return c.PostForm(c.endpoint("hosts"), url.Values{"name": {name}}, &response)
}

func (c *Client) DeleteHost(name string) error {
	response := StatusResponse{}
	return c.Delete(c.endpoint("hosts", name), &response)
}

func (c *Client) AddLocation(id, hostname, path, upstream string) error {
	response := StatusResponse{}
	return c.PostForm(
		c.endpoint("hosts", hostname, "locations"),
		url.Values{
			"id":       {id},
			"path":     {path},
			"upstream": {upstream},
		}, &response)
}

func (c *Client) DeleteLocation(hostname, path string) error {
	response := StatusResponse{}
	return c.Delete(c.endpoint("hosts", hostname, "locations", url.QueryEscape(path)), &response)
}

func (c *Client) AddUpstream(id string) error {
	response := StatusResponse{}
	return c.PostForm(c.endpoint("upstreams"), url.Values{"id": {id}}, &response)
}

func (c *Client) DeleteUpstream(id string) error {
	response := StatusResponse{}
	return c.Delete(c.endpoint("upstream", id), &response)
}

func (c *Client) GetUpstreams() ([]*Upstream, error) {
	upstreams := UpstreamsResponse{}
	err := c.Get(c.endpoint("upstreams"), &upstreams)
	return upstreams.Upstreams, err
}

func (c *Client) AddEndpoint(upstreamId, u string) error {
	response := StatusResponse{}
	return c.PostForm(c.endpoint("upstreams", upstreamId, "endpoints"), url.Values{"url": {u}}, &response)
}

func (c *Client) DeleteEndpoint(upstreamId, u string) error {
	response := StatusResponse{}
	return c.Delete(c.endpoint("upstream", upstreamId, "endpoints", url.QueryEscape(u)), &response)
}

func (c *Client) PostForm(endpoint string, values url.Values, in interface{}) error {
	return c.RoundTripJson(func() (*http.Response, error) {
		return http.PostForm(endpoint, values)
	}, in)
}

func (c *Client) Delete(endpoint string, in interface{}) error {
	return c.RoundTripJson(func() (*http.Response, error) {
		req, err := http.NewRequest("DELETE", endpoint, nil)
		if err != nil {
			return nil, err
		}
		return http.DefaultClient.Do(req)
	}, in)
}

func (c *Client) Get(url string, in interface{}) error {
	return c.RoundTripJson(func() (*http.Response, error) {
		return http.Get(url)
	}, in)
}

type RoundTripFn func() (*http.Response, error)

func (c *Client) RoundTripJson(fn RoundTripFn, in interface{}) error {
	response, err := fn()
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("Error: %s", responseBody)
	}
	if json.Unmarshal(responseBody, in); err != nil {
		return fmt.Errorf("Failed to decode response '%s', error: %", responseBody, err)
	}
	return nil
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

type StatusResponse struct {
	Status string
}
