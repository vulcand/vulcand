package testutils

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/vulcand/oxy/utils"
)

// NewHandler creates a new Server
func NewHandler(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// NewResponder creates a new Server with response
func NewResponder(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	}))
}

// ParseURI is the version of url.ParseRequestURI that panics if incorrect, helpful to shorten the tests
func ParseURI(uri string) *url.URL {
	out, err := url.ParseRequestURI(uri)
	if err != nil {
		panic(err)
	}
	return out
}

// ReqOpts request options
type ReqOpts struct {
	Host    string
	Method  string
	Body    string
	Headers http.Header
	Auth    *utils.BasicAuth
}

// ReqOption request option type
type ReqOption func(o *ReqOpts) error

// Method sets request method
func Method(m string) ReqOption {
	return func(o *ReqOpts) error {
		o.Method = m
		return nil
	}
}

// Host sets request host
func Host(h string) ReqOption {
	return func(o *ReqOpts) error {
		o.Host = h
		return nil
	}
}

// Body sets request body
func Body(b string) ReqOption {
	return func(o *ReqOpts) error {
		o.Body = b
		return nil
	}
}

// Header sets request header
func Header(name, val string) ReqOption {
	return func(o *ReqOpts) error {
		if o.Headers == nil {
			o.Headers = make(http.Header)
		}
		o.Headers.Add(name, val)
		return nil
	}
}

// Headers sets request headers
func Headers(h http.Header) ReqOption {
	return func(o *ReqOpts) error {
		if o.Headers == nil {
			o.Headers = make(http.Header)
		}
		utils.CopyHeaders(o.Headers, h)
		return nil
	}
}

// BasicAuth sets request basic auth
func BasicAuth(username, password string) ReqOption {
	return func(o *ReqOpts) error {
		o.Auth = &utils.BasicAuth{
			Username: username,
			Password: password,
		}
		return nil
	}
}

// MakeRequest create and do a request
func MakeRequest(url string, opts ...ReqOption) (*http.Response, []byte, error) {
	o := &ReqOpts{}
	for _, s := range opts {
		if err := s(o); err != nil {
			return nil, nil, err
		}
	}

	if o.Method == "" {
		o.Method = http.MethodGet
	}
	request, _ := http.NewRequest(o.Method, url, strings.NewReader(o.Body))
	if o.Headers != nil {
		utils.CopyHeaders(request.Header, o.Headers)
	}

	if o.Auth != nil {
		request.Header.Set("Authorization", o.Auth.String())
	}

	if len(o.Host) != 0 {
		request.Host = o.Host
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
			return errors.New("no redirects")
		},
	}
	response, err := client.Do(request)
	if err == nil {
		bodyBytes, errRead := ioutil.ReadAll(response.Body)
		return response, bodyBytes, errRead
	}
	return response, nil, err
}

// Get do a GET request
func Get(url string, opts ...ReqOption) (*http.Response, []byte, error) {
	opts = append(opts, Method(http.MethodGet))
	return MakeRequest(url, opts...)
}
