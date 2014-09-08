package server

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit/tokenbucket"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/testutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin/ratelimit"
	"testing"
)

func TestServer(t *testing.T) { TestingT(t) }

var _ = Suite(&ServerSuite{})

type ServerSuite struct {
	mux    *MuxServer
	lastId int
}

func (s *ServerSuite) SetUpTest(c *C) {
	m, err := NewMuxServerWithOptions(s.lastId, Options{})
	c.Assert(err, IsNil)
	s.mux = m
}

func (s *ServerSuite) TearDownTest(c *C) {
	s.mux.Stop(true)
}

func (s *ServerSuite) TestStartStop(c *C) {
	c.Assert(s.mux.Start(), IsNil)
}

func (s *ServerSuite) TestServerCRUD(c *C) {
	e := newTestServer("Hi, I'm endpoint")
	defer e.Close()

	l, h := makeLocation("localhost", "localhost:31000", e.URL)

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHost(h.Name), IsNil)

	_, _, err := GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestServerDefaultListener(c *C) {
	e := newTestServer("Hi, I'm endpoint")
	defer e.Close()

	defaultListener := &Listener{Protocol: HTTP, Address: Address{"tcp", "localhost:41000"}}

	m, err := NewMuxServerWithOptions(s.lastId, Options{DefaultListener: defaultListener})
	defer m.Stop(true)
	c.Assert(err, IsNil)
	s.mux = m

	l, h := makeLocation("localhost", "localhost:31000", e.URL)

	h.Listeners = []*Listener{}
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(s.mux.Start(), IsNil)
	c.Assert(GETResponse(c, makeURL(l, defaultListener), ""), Equals, "Hi, I'm endpoint")
}

// Test case when you have two hosts on the same domain
func (s *ServerSuite) TestTwoHosts(c *C) {
	e := newTestServer("Hi, I'm endpoint 1")
	defer e.Close()

	e2 := newTestServer("Hi, I'm endpoint 2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	l2, h2 := makeLocation("otherhost", "localhost:31000", e2.URL)
	c.Assert(s.mux.UpsertLocation(h2, l2), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint 1")
	c.Assert(GETResponse(c, makeURL(l, h2.Listeners[0]), "otherhost"), Equals, "Hi, I'm endpoint 2")
}

func (s *ServerSuite) TestServerListenerCRUD(c *C) {
	e := newTestServer("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	h.Listeners = append(h.Listeners, &Listener{Id: "l2", Protocol: HTTP, Address: Address{"tcp", "localhost:31001"}})

	s.mux.AddHostListener(h, h.Listeners[1])

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[1]), ""), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHostListener(h, h.Listeners[1].Id), IsNil)

	_, _, err := GET(makeURL(l, h.Listeners[1]), "")
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestServerHTTPSCRUD(c *C) {
	e := newTestServer("Hi, I'm endpoint")
	defer e.Close()

	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	h.Cert = &Certificate{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteHost(h.Name), IsNil)

	_, _, err := GET(makeURL(l, h.Listeners[0]), "")
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestLiveCertUpdate(c *C) {
	e := newTestServer("Hi, I'm endpoint")
	defer e.Close()
	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	h.Cert = &Certificate{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertHost(h), IsNil)
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint")

	h.Cert = &Certificate{Key: localhostKey2, Cert: localhostCert2}
	c.Assert(s.mux.UpdateHostCert(h.Name, h.Cert), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestSNI(c *C) {
	e := newTestServer("Hi, I'm endpoint 1")
	defer e.Close()

	e2 := newTestServer("Hi, I'm endpoint 2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)
	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	h.Cert = &Certificate{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	l2, h2 := makeLocation("otherhost", "localhost:31000", e2.URL)
	h2.Cert = &Certificate{Key: localhostKey2, Cert: localhostCert2}
	h2.Listeners[0].Protocol = HTTPS
	h2.Options.Default = true

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)
	c.Assert(s.mux.UpsertLocation(h2, l2), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint 1")
	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), "otherhost"), Equals, "Hi, I'm endpoint 2")

	s.mux.DeleteHost(h2.Name)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint 1")

	response, _, err := GET(makeURL(l, h2.Listeners[0]), "otherhost")
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Not(Equals), http.StatusOK)
}

func (s *ServerSuite) TestHijacking(c *C) {
	e := newTestServer("Hi, I'm endpoint 1")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)
	h.Cert = &Certificate{Key: localhostKey, Cert: localhostCert}
	h.Listeners[0].Protocol = HTTPS

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint 1")

	mux2, err := NewMuxServerWithOptions(s.lastId, Options{})
	c.Assert(err, IsNil)

	e2 := newTestServer("Hi, I'm endpoint 2")
	defer e2.Close()

	l2, h2 := makeLocation("localhost", "localhost:31000", e2.URL)
	h2.Cert = &Certificate{Key: localhostKey2, Cert: localhostCert2}
	h2.Listeners[0].Protocol = HTTPS

	c.Assert(mux2.UpsertLocation(h2, l2), IsNil)
	c.Assert(mux2.HijackListenersFrom(s.mux), IsNil)

	c.Assert(mux2.Start(), IsNil)
	s.mux.Stop(true)
	defer mux2.Stop(true)

	c.Assert(GETResponse(c, makeURL(l2, h2.Listeners[0]), ""), Equals, "Hi, I'm endpoint 2")
}

func (s *ServerSuite) TestLocationProperties(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", "http://localhost:12345")
	l.Middlewares = []*MiddlewareInstance{
		makeRateLimit("rl1", 100, "client.ip", 200, 10, l),
	}
	l.Options = LocationOptions{
		Timeouts: LocationTimeouts{
			Dial: "14s",
		},
	}
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	// Make sure location is here
	loc := s.mux.getLocation(h.Name, l.Id)
	c.Assert(loc, NotNil)
	c.Assert(loc.GetOptions().Timeouts.Dial, Equals, time.Second*14)

	// Make sure the endpoint has been added to the location
	lb := s.mux.getLocationLB(h.Name, l.Id)
	c.Assert(lb, NotNil)

	// Check that endpoint is here
	endpoints := lb.GetEndpoints()
	c.Assert(len(endpoints), Equals, 1)
	assertSameEndpoints(c, endpoints, l.Upstream.Endpoints)

	// Make sure connection limit and rate limit are here as well
	chain := loc.GetMiddlewareChain()
	c.Assert(chain.Get("ratelimit.rl1"), NotNil)

	// Delete the location
	c.Assert(s.mux.DeleteLocation(h, l.Id), IsNil)

	// Make sure it's no longer in the proxy
	loc = s.mux.getLocation(h.Name, l.Id)
	c.Assert(loc, IsNil)
}

func (s *ServerSuite) TestUpdateLocationOptions(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", "http://localhost:12345")
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	l.Options = LocationOptions{
		Timeouts: LocationTimeouts{
			Dial: "7s",
		},
		FailoverPredicate: "IsNetworkError",
	}
	c.Assert(s.mux.UpdateLocationOptions(h, l), IsNil)

	lo := s.mux.getLocation(h.Name, l.Id)
	c.Assert(lo.GetOptions().ShouldFailover, NotNil)
	c.Assert(lo.GetOptions().Timeouts.Dial, Equals, time.Second*7)
}

func (s *ServerSuite) TestTrieRoutes(c *C) {
	e1 := newTestServer("Hi, I'm endpoint 1")
	defer e1.Close()

	e2 := newTestServer("Hi, I'm endpoint 2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l1, h1 := makeLocation("localhost", "localhost:31000", e1.URL)
	l1.Path = `TrieRoute("/loc/path1")`
	l1.Id = "loc1"

	l2, h2 := makeLocation("localhost", "localhost:31000", e2.URL)
	l2.Path = `TrieRoute("/loc/path2")`
	l2.Id = "loc2"

	c.Assert(s.mux.UpsertLocation(h1, l1), IsNil)
	c.Assert(s.mux.UpsertLocation(h2, l2), IsNil)

	c.Assert(GETResponse(c, "http://localhost:31000/loc/path1", ""), Equals, "Hi, I'm endpoint 1")
	c.Assert(GETResponse(c, "http://localhost:31000/loc/path2", ""), Equals, "Hi, I'm endpoint 2")
}

func (s *ServerSuite) TestUpdateLocationUpstream(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	e1 := newTestServer("1")
	defer e1.Close()

	e2 := newTestServer("2")
	defer e2.Close()

	e3 := newTestServer("3")
	defer e3.Close()

	h := &Host{
		Name:      "localhost",
		Listeners: []*Listener{&Listener{Protocol: HTTP, Address: Address{"tcp", "localhost:31000"}}},
	}
	up1 := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				Url: e1.URL,
			},
			{
				Url: e2.URL,
			},
		},
	}

	up2 := &Upstream{
		Id: "up2",
		Endpoints: []*Endpoint{
			{
				Url: e2.URL,
			},
			{
				Url: e3.URL,
			},
		},
	}

	l := &Location{
		Hostname: h.Name,
		Path:     "/loc1",
		Id:       "loc1",
		Upstream: up1,
	}

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	// Make sure the endpoint has been added to the location
	lb := s.mux.getLocationLB(h.Name, l.Id)
	c.Assert(lb, NotNil)

	assertSameEndpoints(c, lb.GetEndpoints(), up1.Endpoints)

	responseSet := make(map[string]bool)
	responseSet[GETResponse(c, "http://localhost:31000/loc1", "")] = true
	responseSet[GETResponse(c, "http://localhost:31000/loc1", "")] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true, "2": true})

	l.Upstream = up2

	c.Assert(s.mux.UpdateLocationUpstream(h, l), IsNil)

	assertSameEndpoints(c, lb.GetEndpoints(), up2.Endpoints)

	responseSet = make(map[string]bool)
	responseSet[GETResponse(c, "http://localhost:31000/loc1", "")] = true
	responseSet[GETResponse(c, "http://localhost:31000/loc1", "")] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"2": true, "3": true})
}

func (s *ServerSuite) TestUpstreamEndpointCRUD(c *C) {
	e1 := newTestServer("1")
	defer e1.Close()

	e2 := newTestServer("2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e1.URL)

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	lb := s.mux.getLocationLB(h.Name, l.Id)
	c.Assert(lb, NotNil)

	// Endpoints are taken from the upstream
	up := l.Upstream
	assertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "1")

	// Add some endpoints to location
	newEndpoint := &Endpoint{
		Id:  e2.URL,
		Url: e2.URL,
	}
	up.Endpoints = append(up.Endpoints, newEndpoint)

	c.Assert(s.mux.UpsertEndpoint(up, newEndpoint, []*Location{l}), IsNil)

	// Endpoints have been updated in the load balancer
	assertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints)

	// And actually work
	responseSet := make(map[string]bool)
	responseSet[GETResponse(c, makeURL(l, h.Listeners[0]), "")] = true
	responseSet[GETResponse(c, makeURL(l, h.Listeners[0]), "")] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true, "2": true})

	up.Endpoints = up.Endpoints[:1]
	c.Assert(s.mux.DeleteEndpoint(up, newEndpoint.Id, []*Location{l}), IsNil)

	assertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints)

	// And actually work
	responseSet = make(map[string]bool)
	responseSet[GETResponse(c, makeURL(l, h.Listeners[0]), "")] = true
	responseSet[GETResponse(c, makeURL(l, h.Listeners[0]), "")] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true})
}

func (s *ServerSuite) TestUpstreamAddBadEndpoint(c *C) {
	e1 := newTestServer("1")
	defer e1.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e1.URL)

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	lb := s.mux.getLocationLB(h.Name, l.Id)
	c.Assert(lb, NotNil)

	// Endpoints are taken from the upstream
	up := l.Upstream
	assertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "1")

	// Add some endpoints to location
	newEndpoint := &Endpoint{
		Url: "http: local-host :500",
	}
	up.Endpoints = append(up.Endpoints, newEndpoint)

	c.Assert(s.mux.UpsertEndpoint(up, newEndpoint, []*Location{l}), NotNil)

	// Endpoints have not been updated in the load balancer
	assertSameEndpoints(c, lb.GetEndpoints(), up.Endpoints[:1])
}

func (s *ServerSuite) TestUpstreamUpdateEndpoint(c *C) {
	e1 := newTestServer("1")
	defer e1.Close()

	e2 := newTestServer("2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e1.URL)

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)
	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "1")

	ep := l.Upstream.Endpoints[0]
	ep.Url = e2.URL

	c.Assert(s.mux.UpsertEndpoint(l.Upstream, ep, []*Location{l}), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "2")
}

func (s *ServerSuite) TestUpdateRateLimit(c *C) {
	l, h := makeLocation("localhost", "localhost:31000", "http://localhost:32000")
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	rl := makeRateLimit("rl1", 100, "client.ip", 200, 10, l)

	c.Assert(s.mux.UpsertLocationMiddleware(h, l, rl), IsNil)

	loc := s.mux.getLocation(h.Name, l.Id)
	c.Assert(loc, NotNil)

	// Make sure connection limit and rate limit are here as well
	chain := loc.GetMiddlewareChain()
	limiter := chain.Get("ratelimit.rl1").(*tokenbucket.TokenLimiter)
	c.Assert(limiter.GetRate().Units, Equals, int64(100))
	c.Assert(limiter.GetRate().Period, Equals, time.Second*time.Duration(10))
	c.Assert(limiter.GetBurst(), Equals, int64(200))

	// Update the rate limit
	rl = makeRateLimit("rl1", 12, "client.ip", 20, 3, l)
	c.Assert(s.mux.UpsertLocationMiddleware(h, l, rl), IsNil)

	// Make sure the changes have taken place
	limiter = chain.Get("ratelimit.rl1").(*tokenbucket.TokenLimiter)
	c.Assert(limiter.GetRate().Units, Equals, int64(12))
	c.Assert(limiter.GetRate().Period, Equals, time.Second*time.Duration(3))
	c.Assert(limiter.GetBurst(), Equals, int64(20))
}

func (s *ServerSuite) TestRateLimitCRUD(c *C) {
	l, h := makeLocation("localhost", "localhost:31000", "http://localhost:32000")
	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	rl := makeRateLimit("r1", 10, "client.ip", 1, 1, l)
	rl2 := makeRateLimit("r2", 10, "client.ip", 1, 1, l)

	c.Assert(s.mux.UpsertLocationMiddleware(h, l, rl), IsNil)
	c.Assert(s.mux.UpsertLocationMiddleware(h, l, rl2), IsNil)

	loc := s.mux.getLocation(h.Name, l.Id)
	c.Assert(loc, NotNil)

	chain := loc.GetMiddlewareChain()
	c.Assert(chain.Get("ratelimit.r1"), NotNil)
	c.Assert(chain.Get("ratelimit.r2"), NotNil)

	c.Assert(s.mux.DeleteLocationMiddleware(h, l, rl.Type, rl.Id), IsNil)

	c.Assert(chain.Get("ratelimit.r1"), IsNil)
	// Make sure that the other rate limiter is still there
	c.Assert(chain.Get("ratelimit.r2"), NotNil)
}

func (s *ServerSuite) TestUpdateLocationPath(c *C) {
	e := newTestServer("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)

	c.Assert(s.mux.UpsertLocation(h, l), IsNil)

	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint")

	l.Path = `TrieRoute("/hello/path2")`

	c.Assert(s.mux.UpdateLocationPath(h, l, l.Path), IsNil)

	c.Assert(GETResponse(c, "http://localhost:31000/hello/path2", ""), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestUpdateLocationPathCreateLocation(c *C) {
	e := newTestServer("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	l, h := makeLocation("localhost", "localhost:31000", e.URL)

	c.Assert(s.mux.UpdateLocationPath(h, l, l.Path), IsNil)
	c.Assert(GETResponse(c, makeURL(l, h.Listeners[0]), ""), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestConvertPath(c *C) {
	c.Assert(convertPath(`TrieRoute("hello")`), Equals, `TrieRoute("hello")`)
	c.Assert(convertPath(`RegexpRoute("hello")`), Equals, `RegexpRoute("hello")`)
	c.Assert(convertPath(`/hello`), Equals, `RegexpRoute("/hello")`)
}

func makeURL(loc *Location, l *Listener) string {
	return fmt.Sprintf("%s://%s%s", l.Protocol, l.Address.Address, loc.Path)
}

func makeLocation(hostname, listenerAddress, endpointURL string) (*Location, *Host) {
	host := &Host{
		Name:      hostname,
		Listeners: []*Listener{&Listener{Protocol: HTTP, Address: Address{"tcp", listenerAddress}}}}

	upstream := &Upstream{
		Id: "up1",
		Endpoints: []*Endpoint{
			{
				Id:  endpointURL,
				Url: endpointURL,
			},
		},
	}
	location := &Location{
		Hostname: host.Name,
		Path:     "/loc1",
		Id:       "loc1",
		Upstream: upstream,
	}
	return location, host
}

func GETResponse(c *C, url string, host string) string {
	response, body, err := GET(url, host)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	return string(body)
}

func GET(url string, host string) (*http.Response, []byte, error) {
	request, _ := http.NewRequest("GET", url, strings.NewReader(""))
	if len(host) != 0 {
		request.Host = host
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

	client := &http.Client{Transport: tr}
	response, err := client.Do(request)
	if err == nil {
		bodyBytes, err := ioutil.ReadAll(response.Body)
		return response, bodyBytes, err
	}
	return response, nil, err
}

func makeRateLimit(id string, rate int, variable string, burst int64, periodSeconds int, loc *Location) *MiddlewareInstance {
	rl, err := ratelimit.NewRateLimit(rate, variable, burst, periodSeconds)
	if err != nil {
		panic(err)
	}
	return &MiddlewareInstance{
		Type:       "ratelimit",
		Id:         id,
		Middleware: rl,
	}
}

func assertSameEndpoints(c *C, a []*roundrobin.WeightedEndpoint, b []*Endpoint) {
	x, y := map[string]bool{}, map[string]bool{}
	for _, e := range a {
		x[e.GetUrl().String()] = true
	}

	for _, e := range b {
		y[e.Url] = true
	}
	c.Assert(x, DeepEquals, y)
}

func newTestServer(response string) *httptest.Server {
	return NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	})
}

// localhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at the last second of 2049 (the end
// of ASN.1 time).
// generated from src/pkg/crypto/tls:
// go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAN55NcYKZeInyTuhcCwFMhDHCmwaIUSdtXdcbItRB/yfXGBhiex0
0IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEAAQJBAQdUx66rfh8sYsgfdcvV
NoafYpnEcB5s4m/vSVe6SU7dCK6eYec9f9wpT353ljhDUHq3EbmE4foNzJngh35d
AekCIQDhRQG5Li0Wj8TM4obOnnXUXf1jRv0UkzE9AHWLG5q3AwIhAPzSjpYUDjVW
MCUXgckTpKCuGwbJk7424Nb8bLzf3kllAiA5mUBgjfr/WtFSJdWcPQ4Zt9KTMNKD
EUO0ukpTwEIl6wIhAMbGqZK3zAAFdq8DD2jPx+UJXnh0rnOkZBzDtJ6/iN69AiEA
1Aq8MJgTaYsDQWyU/hDq5YkDJc9e9DSCvUIzqxQWMQE=
-----END RSA PRIVATE KEY-----`)

var localhostCert2 = []byte(`-----BEGIN CERTIFICATE-----
MIIBizCCATegAwIBAgIRAL3EdJdBpGqcIy7kqCul6qIwCwYJKoZIhvcNAQELMBIx
EDAOBgNVBAoTB0FjbWUgQ28wIBcNNzAwMTAxMDAwMDAwWhgPMjA4NDAxMjkxNjAw
MDBaMBIxEDAOBgNVBAoTB0FjbWUgQ28wXDANBgkqhkiG9w0BAQEFAANLADBIAkEA
zAy3eIgjhro/wksSVgN+tZMxNbETDPgndYpIVSMMGHRXid71Zit8R5jJg8GZhWOs
2GXAZVZIJy634mODg5Xs8QIDAQABo2gwZjAOBgNVHQ8BAf8EBAMCAKQwEwYDVR0l
BAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zAuBgNVHREEJzAlggtleGFt
cGxlLmNvbYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATALBgkqhkiG9w0BAQsDQQA2
NW/PChPgBPt4q4ATTDDmoLoWjY8Vrp++6Wtue1YQBfEyvGWTFibNLD7FFodIPg/a
5LgeVKZTukSJX31lVCBm
-----END CERTIFICATE-----`)

var localhostKey2 = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBAMwMt3iII4a6P8JLElYDfrWTMTWxEwz4J3WKSFUjDBh0V4ne9WYr
fEeYyYPBmYVjrNhlwGVWSCcut+Jjg4OV7PECAwEAAQJAYHjOsZzj9wnNpUWrCKGk
YaKSzIjIsgQNW+QiKKZmTJS0rCJnUXUz8nSyTnS5rYd+CqOlFDXzpDbcouKGLOn5
BQIhAOtwl7+oebSLYHvznksQg66yvRxULfQTJS7aIKHNpDTPAiEA3d5gllV7EuGq
oqcbLwrFrGJ4WflasfeLpcDXuOR7sj8CIQC34IejuADVcMU6CVpnZc5yckYgCd6Z
8RnpLZKuy9yjIQIgYsykNk3agI39bnD7qfciD6HJ9kcUHCwgA6/cYHlenAECIQDZ
H4E4GFiDetx8ZOdWq4P7YRdIeepSvzPeOEv2sfsItg==
-----END RSA PRIVATE KEY-----`)
