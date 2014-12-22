// This package contains the reverse proxy that implements http.HandlerFunc
package vulcan

import (
	"io"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/mailgun/log"
	"github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcan/route"
)

type Proxy struct {
	// Router selects a location for each request
	router route.Router
	// Options like ErrorFormatter
	options Options
	// Counter that is used to provide unique identifiers for requests
	lastRequestId int64
}

type Options struct {
	// Takes a status code and formats it into proxy response
	ErrorFormatter errors.Formatter
}

// Accepts requests, round trips it to the endpoint, and writes back the response.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := p.proxyRequest(w, r)
	if err == nil {
		return
	}

	switch e := err.(type) {
	case *errors.RedirectError:
		// In case if it's redirect error, try the request one more time, but with different URL
		r.URL = e.URL
		r.Host = e.URL.Host
		r.RequestURI = e.URL.String()
		if err := p.proxyRequest(w, r); err != nil {
			p.replyError(err, w, r)
		}
	default:
		p.replyError(err, w, r)
	}
}

// Creates a proxy with a given router
func NewProxy(router route.Router) (*Proxy, error) {
	return NewProxyWithOptions(router, Options{})
}

// Creates reverse proxy that acts like http request handler
func NewProxyWithOptions(router route.Router, o Options) (*Proxy, error) {
	o, err := validateOptions(o)
	if err != nil {
		return nil, err
	}

	p := &Proxy{
		options: o,
		router:  router,
	}
	return p, nil
}

func (p *Proxy) GetRouter() route.Router {
	return p.router
}

// Round trips the request to the selected location and writes back the response
func (p *Proxy) proxyRequest(w http.ResponseWriter, r *http.Request) error {

	// Create a unique request with sequential ids that will be passed to all interfaces.
	req := request.NewBaseRequest(r, atomic.AddInt64(&p.lastRequestId, 1), nil)
	location, err := p.router.Route(req)
	if err != nil {
		return err
	}

	// Router could not find a matching location, we can do nothing else.
	if location == nil {
		log.Errorf("%s failed to route", req)
		return errors.FromStatus(http.StatusBadGateway)
	}

	response, err := location.RoundTrip(req)
	if response != nil {
		netutils.CopyHeaders(w.Header(), response.Header)
		w.WriteHeader(response.StatusCode)
		io.Copy(w, response.Body)
		response.Body.Close()
		return nil
	} else {
		return err
	}
}

// replyError is a helper function that takes error and replies with HTTP compatible error to the client.
func (p *Proxy) replyError(err error, w http.ResponseWriter, req *http.Request) {
	proxyError := convertError(err)
	statusCode, body, contentType := p.options.ErrorFormatter.Format(proxyError)
	w.Header().Set("Content-Type", contentType)
	if proxyError.Headers() != nil {
		netutils.CopyHeaders(w.Header(), proxyError.Headers())
	}
	w.WriteHeader(statusCode)
	w.Write(body)
}

func validateOptions(o Options) (Options, error) {
	if o.ErrorFormatter == nil {
		o.ErrorFormatter = &errors.JsonFormatter{}
	}
	return o, nil
}

func convertError(err error) errors.ProxyError {
	switch e := err.(type) {
	case errors.ProxyError:
		return e
	case net.Error:
		if e.Timeout() {
			return errors.FromStatus(http.StatusRequestTimeout)
		}
	case *netutils.MaxSizeReachedError:
		return errors.FromStatus(http.StatusRequestEntityTooLarge)
	}
	return errors.FromStatus(http.StatusBadGateway)
}
