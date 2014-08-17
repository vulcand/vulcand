// HTTP location with load balancing and pluggable middlewares
package httploc

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/failover"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

// Location with built in failover and load balancing support
type HttpLocation struct {
	// Unique identifier of this location
	id string
	// Transport with customized timeouts
	transport *http.Transport
	// Load balancer controls endpoints for this location
	loadBalancer loadbalance.LoadBalancer
	// Timeouts, failover and other optional settings
	options Options
	// Chain with pluggable middlewares that can intercept the request
	middlewareChain *middleware.MiddlewareChain
	// Chain of observers that watch the request
	observerChain *middleware.ObserverChain
	// Mutex controls the changes on the Transport and connection options
	mutex *sync.RWMutex
}

type Timeouts struct {
	// Socket read timeout (before we receive the first reply header)
	Read time.Duration
	// Socket connect timeout
	Dial time.Duration
	// TLS handshake timeout
	TlsHandshake time.Duration
}

type KeepAlive struct {
	// Keepalive period
	Period time.Duration
	// How many idle connections will be kept per host
	MaxIdleConnsPerHost int
}

// Limits contains various limits one can supply for a location.
type Limits struct {
	MaxMemBodyBytes int64 // Maximum size to keep in memory before buffering to disk
	MaxBodyBytes    int64 // Maximum size of a request body in bytes
}

// Additional options to control this location, such as timeouts
type Options struct {
	Timeouts Timeouts
	// Controls KeepAlive settins for backend servers
	KeepAlive KeepAlive
	// Limits contains various limits one can supply for a location.
	Limits Limits
	// Predicate that defines when requests are allowed to failover
	ShouldFailover failover.Predicate
	// Used in forwarding headers
	Hostname string
	// In this case appends new forward info to the existing header
	TrustForwardHeader bool
	// Time provider (useful for testing purposes)
	TimeProvider timetools.TimeProvider
}

func NewLocation(id string, loadBalancer loadbalance.LoadBalancer) (*HttpLocation, error) {
	return NewLocationWithOptions(id, loadBalancer, Options{})
}

func NewLocationWithOptions(id string, loadBalancer loadbalance.LoadBalancer, o Options) (*HttpLocation, error) {
	if loadBalancer == nil {
		return nil, fmt.Errorf("Provide load balancer")
	}
	o, err := parseOptions(o)
	if err != nil {
		return nil, err
	}

	observerChain := middleware.NewObserverChain()
	observerChain.Add(BalancerId, loadBalancer)

	middlewareChain := middleware.NewMiddlewareChain()
	middlewareChain.Add(RewriterId, -2, &Rewriter{TrustForwardHeader: o.TrustForwardHeader, Hostname: o.Hostname})
	middlewareChain.Add(BalancerId, -1, loadBalancer)

	return &HttpLocation{
		id:              id,
		loadBalancer:    loadBalancer,
		options:         o,
		transport:       newTransport(o),
		middlewareChain: middlewareChain,
		observerChain:   observerChain,
		mutex:           &sync.RWMutex{},
	}, nil
}

func (l *HttpLocation) SetOptions(o Options) error {
	options, err := parseOptions(o)
	if err != nil {
		return err
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if err := l.middlewareChain.Update(RewriterId, -2, &Rewriter{TrustForwardHeader: o.TrustForwardHeader, Hostname: o.Hostname}); err != nil {
		return err
	}
	l.options = options
	l.setTransport(newTransport(options))
	return nil
}

func (l *HttpLocation) GetOptions() Options {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.options
}

func (l *HttpLocation) GetOptionsAndTransport() (Options, *http.Transport) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.options, l.transport
}

func (l *HttpLocation) setTransport(tr *http.Transport) {
	if l.transport != nil {
		go l.transport.CloseIdleConnections()
	}
	l.transport = tr
}

func (l *HttpLocation) GetMiddlewareChain() *middleware.MiddlewareChain {
	return l.middlewareChain
}

func (l *HttpLocation) GetObserverChain() *middleware.ObserverChain {
	return l.observerChain
}

// Round trips the request to one of the endpoints and returns the response.
func (l *HttpLocation) RoundTrip(req request.Request) (*http.Response, error) {
	// Get options and transport as one single read transaction.
	// Options and transport may change if someone calls SetOptions
	o, tr := l.GetOptionsAndTransport()
	originalRequest := req.GetHttpRequest()

	//  Check request size first, if that exceeds the limit, we don't bother reading the request.
	if l.isRequestOverLimit(req) {
		return nil, errors.FromStatus(http.StatusRequestEntityTooLarge)
	}

	// Read the body while keeping this location's limits in mind. This reader controls the maximum bytes
	// to read into memory and disk. This reader returns anerror if the total request size exceeds the
	// prefefined MaxSizeBytes. This can occur if we got chunked request, in this case ContentLength would be set to -1
	// and the reader would be unbounded bufio in the http.Server
	body, err := netutils.NewBodyBufferWithOptions(originalRequest.Body, netutils.BodyBufferOptions{
		MemBufferBytes: o.Limits.MaxMemBodyBytes,
		MaxSizeBytes:   o.Limits.MaxBodyBytes,
	})
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("Empty body")
	}

	// Set request body to buffered reader that can replay the read and execute Seek
	req.SetBody(body)
	// Note that we don't change the original request Body as it's handled by the http server
	defer body.Close()

	for {
		_, err := req.GetBody().Seek(0, 0)
		if err != nil {
			return nil, err
		}

		endpoint, err := l.loadBalancer.NextEndpoint(req)
		if err != nil {
			log.Errorf("Load Balancer failure: %s", err)
			return nil, err
		}

		// Adds headers, changes urls. Note that we rewrite request each time we proxy it to the
		// endpoint, so that each try gets a fresh start
		req.SetHttpRequest(l.copyRequest(req, endpoint))

		// In case if error is not nil, we allow load balancer to choose the next endpoint
		// e.g. to do request failover. Nil error means that we got proxied the request successfully.
		response, err := l.proxyToEndpoint(tr, &o, endpoint, req)
		if o.ShouldFailover(req) {
			continue
		} else {
			return response, err
		}
	}
	log.Errorf("All endpoints failed!")
	return nil, fmt.Errorf("All endpoints failed")
}

func (l *HttpLocation) GetLoadBalancer() loadbalance.LoadBalancer {
	return l.loadBalancer
}

func (l *HttpLocation) GetId() string {
	return l.id
}

// Unwind middlewares iterator in reverse order
func (l *HttpLocation) unwindIter(it *middleware.MiddlewareIter, req request.Request, a request.Attempt) {
	for v := it.Prev(); v != nil; v = it.Prev() {
		v.ProcessResponse(req, a)
	}
}

func (l *HttpLocation) isRequestOverLimit(req request.Request) bool {
	if l.options.Limits.MaxBodyBytes <= 0 {
		return false
	}
	return req.GetHttpRequest().ContentLength > l.options.Limits.MaxBodyBytes
}

// Proxy the request to the given endpoint, execute observers and middlewares chains
func (l *HttpLocation) proxyToEndpoint(tr *http.Transport, o *Options, endpoint endpoint.Endpoint, req request.Request) (*http.Response, error) {

	a := &request.BaseAttempt{Endpoint: endpoint}

	l.observerChain.ObserveRequest(req)
	defer l.observerChain.ObserveResponse(req, a)
	defer req.AddAttempt(a)

	it := l.middlewareChain.GetIter()
	defer l.unwindIter(it, req, a)

	for v := it.Next(); v != nil; v = it.Next() {
		a.Response, a.Error = v.ProcessRequest(req)
		if a.Response != nil || a.Error != nil {
			// Move the iterator forward to count it again once we unwind the chain
			it.Next()
			log.Errorf("Midleware intercepted request with response=%s, error=%s", a.Response.Status, a.Error)
			return a.Response, a.Error
		}
	}

	// Forward the request and mirror the response
	start := o.TimeProvider.UtcNow()
	a.Response, a.Error = tr.RoundTrip(req.GetHttpRequest())
	a.Duration = o.TimeProvider.UtcNow().Sub(start)
	return a.Response, a.Error
}

func (l *HttpLocation) copyRequest(r request.Request, endpoint endpoint.Endpoint) *http.Request {
	req := r.GetHttpRequest()

	outReq := new(http.Request)
	*outReq = *req // includes shallow copies of maps, but we handle this below

	// Set the body to the enhanced body that can be re-read multiple times and buffered to disk
	outReq.Body = r.GetBody()

	outReq.URL.Scheme = endpoint.GetUrl().Scheme
	outReq.URL.Host = endpoint.GetUrl().Host
	outReq.URL.RawQuery = req.URL.RawQuery

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1

	// Overwrite close flag so we can keep persistent connection for the backend servers
	outReq.Close = false

	outReq.Header = make(http.Header)
	netutils.CopyHeaders(outReq.Header, req.Header)
	return outReq
}

// Standard dial and read timeouts, can be overriden when supplying location
const (
	DefaultHttpReadTimeout     = time.Duration(10) * time.Second
	DefaultHttpDialTimeout     = time.Duration(10) * time.Second
	DefaultTlsHandshakeTimeout = time.Duration(10) * time.Second
	DefaultKeepAlivePeriod     = time.Duration(30) * time.Second
	DefaultMaxIdleConnsPerHost = 2
)

func parseOptions(o Options) (Options, error) {
	if o.Limits.MaxMemBodyBytes <= 0 {
		o.Limits.MaxMemBodyBytes = netutils.DefaultMemBufferBytes
	}
	if o.Timeouts.Read <= time.Duration(0) {
		o.Timeouts.Read = DefaultHttpReadTimeout
	}
	if o.Timeouts.Dial <= time.Duration(0) {
		o.Timeouts.Dial = DefaultHttpDialTimeout
	}
	if o.Timeouts.TlsHandshake <= time.Duration(0) {
		o.Timeouts.TlsHandshake = DefaultTlsHandshakeTimeout
	}
	if o.KeepAlive.Period <= time.Duration(0) {
		o.KeepAlive.Period = DefaultKeepAlivePeriod
	}
	if o.KeepAlive.MaxIdleConnsPerHost <= 0 {
		o.KeepAlive.MaxIdleConnsPerHost = DefaultMaxIdleConnsPerHost
	}

	if o.Hostname == "" {
		h, err := os.Hostname()
		if err == nil {
			o.Hostname = h
		}
	}
	if o.TimeProvider == nil {
		o.TimeProvider = &timetools.RealTime{}
	}
	if o.ShouldFailover == nil {
		// Failover on errors for 2 times maximum on GET requests only.
		o.ShouldFailover = failover.And(failover.AttemptsLe(2), failover.IsNetworkError, failover.RequestMethodEq("GET"))
	}
	return o, nil
}

func newTransport(o Options) *http.Transport {
	return &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   o.Timeouts.Dial,
			KeepAlive: o.KeepAlive.Period,
		}).Dial,
		ResponseHeaderTimeout: o.Timeouts.Read,
		TLSHandshakeTimeout:   o.Timeouts.TlsHandshake,
	}
}

const (
	BalancerId = "__loadBalancer"
	RewriterId = "__rewriter"
)
