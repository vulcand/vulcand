package main

import (
	"encoding/json"
	"fmt"
	. "github.com/mailgun/vulcand/proxy"
	"io/ioutil"
	"net/http"
	"strings"
)

const CurrentVersion = "v1"

type Client struct {
	Addr string
}

func NewClient(addr string) *Client {
	return &Client{Addr: addr}
}

func (c *Client) GetServers() ([]Server, error) {
	servers := Servers{}
	err := c.GetJson(c.endpoint("servers"), &servers)
	return servers.Servers, err
}

func (c *Client) GetJson(url string, in interface{}) error {
	out, err := c.Get(url)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, in)
}

func (c *Client) Get(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return ioutil.ReadAll(response.Body)
}

func (c *Client) endpoint(params ...string) string {
	return fmt.Sprintf("%s/%s/%s", c.Addr, CurrentVersion, strings.Join(params, "/"))
}

type Servers struct {
	Servers []Server
}
