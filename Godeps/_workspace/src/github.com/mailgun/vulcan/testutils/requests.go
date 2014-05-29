package testutils

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"io/ioutil"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

func Get(c *gocheck.C, requestUrl string, header http.Header, body string) (*http.Response, []byte) {
	request, _ := http.NewRequest("GET", requestUrl, strings.NewReader(body))
	netutils.CopyHeaders(request.Header, header)
	request.Close = true
	// the HTTP lib treats Host as a special header.  it only respects the value on req.Host, and ignores
	// values in req.Headers
	if header.Get("Host") != "" {
		request.Host = header.Get("Host")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		c.Fatalf("Get: %v", err)
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Fatalf("Get body failed: %v", err)
	}
	return response, bodyBytes
}

func Post(c *gocheck.C, requestUrl string, header http.Header, body url.Values) (*http.Response, []byte) {
	request, _ := http.NewRequest("POST", requestUrl, strings.NewReader(body.Encode()))
	netutils.CopyHeaders(request.Header, header)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Close = true
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		c.Fatalf("Post: %v", err)
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.Fatalf("Post body failed: %v", err)
	}
	return response, bodyBytes
}

type WebHandler func(http.ResponseWriter, *http.Request)

func NewTestServer(handler WebHandler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handler))
}
