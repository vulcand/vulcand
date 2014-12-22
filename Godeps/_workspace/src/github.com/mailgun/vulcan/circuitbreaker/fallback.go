package circuitbreaker

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/request"
)

type Response struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

func (re *Response) getHTTPResponse(r request.Request) *http.Response {
	return netutils.NewHttpResponse(r.GetHttpRequest(), re.StatusCode, re.Body, re.ContentType)
}

type ResponseFallback struct {
	r Response
}

func NewResponseFallback(r Response) (*ResponseFallback, error) {
	if r.StatusCode == 0 {
		return nil, fmt.Errorf("response code should not be 0")
	}
	return &ResponseFallback{r: r}, nil
}

func (f *ResponseFallback) ProcessRequest(r request.Request) (*http.Response, error) {
	return f.r.getHTTPResponse(r), nil
}

func (f *ResponseFallback) ProcessResponse(r request.Request, a request.Attempt) {
}

type Redirect struct {
	URL string
}

type RedirectFallback struct {
	u *url.URL
}

func NewRedirectFallback(r Redirect) (*RedirectFallback, error) {
	u, err := netutils.ParseUrl(r.URL)
	if err != nil {
		return nil, err
	}
	return &RedirectFallback{u: u}, nil
}

func (f *RedirectFallback) ProcessRequest(r request.Request) (*http.Response, error) {
	return nil, &errors.RedirectError{URL: netutils.CopyUrl(f.u)}
}

func (f *RedirectFallback) ProcessResponse(r request.Request, a request.Attempt) {
}
