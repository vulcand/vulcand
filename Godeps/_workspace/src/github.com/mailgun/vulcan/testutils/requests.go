package testutils

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/mailgun/vulcan/netutils"
)

type Opts struct {
	Host    string
	Method  string
	Body    string
	Headers http.Header
}

func MakeRequest(url string, opts Opts) (*http.Response, []byte, error) {
	method := "GET"
	if opts.Method != "" {
		method = opts.Method
	}
	request, _ := http.NewRequest(method, url, strings.NewReader(opts.Body))
	if opts.Headers != nil {
		netutils.CopyHeaders(request.Header, opts.Headers)
	}

	if len(opts.Host) != 0 {
		request.Host = opts.Host
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

	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("No redirects")
		},
	}
	response, err := client.Do(request)
	if err == nil {
		bodyBytes, err := ioutil.ReadAll(response.Body)
		return response, bodyBytes, err
	}
	return response, nil, err
}

func GET(url string, o Opts) (*http.Response, []byte, error) {
	o.Method = "GET"
	return MakeRequest(url, o)
}

type WebHandler func(http.ResponseWriter, *http.Request)

func NewTestServer(handler WebHandler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(handler))
}

func NewTestResponder(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	}))
}
