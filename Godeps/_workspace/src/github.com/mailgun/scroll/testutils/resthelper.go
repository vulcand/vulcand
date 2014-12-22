package testutils

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/mailgun/scroll"
)

type RestHelper struct{}

func (h *RestHelper) Get(c *C, url string) scroll.Response {
	response, err := http.Get(url)
	if err != nil {
		c.Fatal(err)
	}
	return h.parseResponse(c, response)
}

func (h *RestHelper) Post(c *C, url string, data url.Values) scroll.Response {
	response, err := http.PostForm(url, data)
	if err != nil {
		c.Fatal(err)
	}
	return h.parseResponse(c, response)
}

func (h *RestHelper) PostJSON(c *C, url, data string) scroll.Response {
	request, err := http.NewRequest("POST", url, strings.NewReader(data))
	if err != nil {
		c.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		c.Fatal(err)
	}
	return h.parseResponse(c, response)
}

func (h *RestHelper) Delete(c *C, url string) scroll.Response {
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		c.Fatal(err)
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		c.Fatal(err)
	}
	return h.parseResponse(c, response)
}

func (h *RestHelper) parseResponse(c *C, response *http.Response) scroll.Response {
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Fatal(err)
	}

	parsedResponse := scroll.Response{}
	err = json.Unmarshal(responseBytes, &parsedResponse)
	if err != nil {
		c.Fatal(err)
	}

	return parsedResponse
}
