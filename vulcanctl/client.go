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

func (c *Client) AddHost(name string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.PostForm(c.endpoint("hosts"), url.Values{"name": {name}}, &response)
}

func (c *Client) DeleteHost(name string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.Delete(c.endpoint("hosts", name), &response)
}

func (c *Client) AddLocation(hostname, id, path, upstream string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.PostForm(
		c.endpoint("hosts", hostname, "locations"),
		url.Values{
			"id":       {id},
			"path":     {path},
			"upstream": {upstream},
		}, &response)
}

func (c *Client) DeleteLocation(hostname, id string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.Delete(c.endpoint("hosts", hostname, "locations", url.QueryEscape(id)), &response)
}

func (c *Client) UpdateLocationUpstream(hostname, location, upstream string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.PutForm(c.endpoint("hosts", hostname, "locations", location), url.Values{"upstream": {upstream}}, &response)
}

func (c *Client) AddUpstream(id string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.PostForm(c.endpoint("upstreams"), url.Values{"id": {id}}, &response)
}

func (c *Client) DeleteUpstream(id string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.Delete(c.endpoint("upstreams", id), &response)
}

func (c *Client) GetUpstreams() ([]*Upstream, error) {
	upstreams := UpstreamsResponse{}
	err := c.Get(c.endpoint("upstreams"), &upstreams)
	return upstreams.Upstreams, err
}

func (c *Client) AddEndpoint(upstreamId, id, u string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.PostForm(c.endpoint("upstreams", upstreamId, "endpoints"), url.Values{"url": {u}, "id": {id}}, &response)
}

func (c *Client) DeleteEndpoint(upstreamId, id string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.Delete(c.endpoint("upstreams", upstreamId, "endpoints", id), &response)
}

func (c *Client) AddRateLimit(hostname, location, id, variable, requests, seconds, burst string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.PostForm(
		c.endpoint("hosts", hostname, "locations", location, "limits", "rates"),
		url.Values{
			"id":       {id},
			"variable": {variable},
			"requests": {requests},
			"seconds":  {seconds},
			"burst":    {burst},
		}, &response)
}

func (c *Client) UpdateRateLimit(hostname, location, id, variable, requests, seconds, burst string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.PutForm(
		c.endpoint("hosts", hostname, "locations", location, "limits", "rates", id),
		url.Values{
			"id":       {id},
			"variable": {variable},
			"requests": {requests},
			"seconds":  {seconds},
			"burst":    {burst},
		}, &response)
}

func (c *Client) DeleteRateLimit(hostname, location, id string) (*StatusResponse, error) {
	response := StatusResponse{}
	return &response, c.Delete(c.endpoint("hosts", hostname, "locations", location, "limits", "rates", id), &response)
}

func (c *Client) PutForm(endpoint string, values url.Values, in interface{}) error {
	return c.RoundTripJson(func() (*http.Response, error) {
		req, err := http.NewRequest("PUT", endpoint, strings.NewReader(values.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return http.DefaultClient.Do(req)
	}, in)
}

func (c *Client) PostForm(endpoint string, values url.Values, in interface{}) error {
	return c.RoundTripJson(func() (*http.Response, error) {
		return http.PostForm(endpoint, values)
	}, in)
}

func (c *Client) Delete(endpoint string, in interface{}) error {
	fmt.Println(endpoint)
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
	Message string
}
