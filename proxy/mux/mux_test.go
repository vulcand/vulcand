package mux

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vulcand/oxy/testutils"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/proxy"
	"github.com/vulcand/vulcand/stapler"
	. "github.com/vulcand/vulcand/testutils"
	. "gopkg.in/check.v1"
)

func TestServer(t *testing.T) { TestingT(t) }

var _ = Suite(&ServerSuite{})

type ServerSuite struct {
	mux    *mux
	lastId int
	st     stapler.Stapler
}

func (s *ServerSuite) SetUpTest(c *C) {
	st := stapler.New()
	m, err := New(s.lastId, st, proxy.Options{})
	c.Assert(err, IsNil)
	s.mux = m
	s.st = st
}

func (s *ServerSuite) TearDownTest(c *C) {
	s.mux.Stop(true)
}
func (s *ServerSuite) TestStartStop(c *C) {
	c.Assert(s.mux.Start(), IsNil)
}

func (s *ServerSuite) TestBackendCRUD(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:11300", Route: `Path("/")`, URL: e.URL})

	c.Assert(s.mux.UpsertBackend(b.B), IsNil)
	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteListener(b.LK), IsNil)

	_, _, err := testutils.Get(b.FrontendURL("/"))
	c.Assert(err, NotNil)
}

// Here we upsert only server that creates backend with default settings
func (s *ServerSuite) TestServerCRUD(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:11300", Route: `Path("/")`, URL: e.URL})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteListener(b.LK), IsNil)

	_, _, err := testutils.Get(b.FrontendURL("/"))
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestServerUpsertSame(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:11300", Route: `Path("/")`, URL: e.URL})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestServerDefaultListener(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:41000", Route: `Path("/")`, URL: e.URL})

	m, err := New(s.lastId, s.st, proxy.Options{DefaultListener: &b.L})
	defer m.Stop(true)
	c.Assert(err, IsNil)
	s.mux = m

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

// Test case when you have two hosts on the same socket
func (s *ServerSuite) TestTwoHosts(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint 1")
	defer e.Close()

	e2 := testutils.NewResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{Addr: "localhost:41000", Route: `Host("localhost") && Path("/")`, URL: e.URL})
	b2 := MakeBatch(Batch{Addr: "localhost:41000", Route: `Host("otherhost") && Path("/")`, URL: e2.URL})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertServer(b2.BK, b2.S), IsNil)

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertFrontend(b2.F), IsNil)

	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/"), testutils.Host("localhost")), Equals, "Hi, I'm endpoint 1")
	c.Assert(GETResponse(c, b.FrontendURL("/"), testutils.Host("otherhost")), Equals, "Hi, I'm endpoint 2")
}

func (s *ServerSuite) TestListenerCRUD(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{Addr: "localhost:41000", Route: `Host("localhost") && Path("/")`, URL: e.URL})
	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")

	l2 := MakeListener("localhost:41001", engine.HTTP)
	c.Assert(s.mux.UpsertListener(l2), IsNil)

	c.Assert(GETResponse(c, MakeURL(l2, "/")), Equals, "Hi, I'm endpoint")

	c.Assert(s.mux.DeleteListener(engine.ListenerKey{Id: l2.Id}), IsNil)

	_, _, err := testutils.Get(MakeURL(l2, "/"))
	c.Assert(err, NotNil)
}

func (s *ServerSuite) TestListenerScope(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{Addr: "localhost:41000", Route: `Path("/")`, URL: e.URL})

	b.L.Scope = `Host("localhost")`
	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
	re, _, err := testutils.Get(b.FrontendURL("/"), testutils.Host("otherhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusNotFound)
}

func (s *ServerSuite) TestListenerScopeUpdate(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{Addr: "localhost:41000", Route: `Path("/")`, URL: e.URL})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	re, body, err := testutils.Get(b.FrontendURL("/"), testutils.Host("otherhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, "Hi, I'm endpoint")

	b.L.Scope = `Host("localhost")`
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	re, body, err = testutils.Get(b.FrontendURL("/"), testutils.Host("localhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusOK)
	c.Assert(string(body), Equals, "Hi, I'm endpoint")

	re, _, err = testutils.Get(b.FrontendURL("/"), testutils.Host("otherhost"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusNotFound)
}

func (s *ServerSuite) TestServerNoBody(c *C) {
	e := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	})
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e.URL,
	})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	re, _, err := testutils.Get(b.FrontendURL("/"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusNotModified)
}

func (s *ServerSuite) TestServerHTTPS(c *C) {
	var req *http.Request
	e := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		req = r
		w.Write([]byte("hi https"))
	})
	defer e.Close()

	b := MakeBatch(Batch{
		Addr:     "localhost:41000",
		Route:    `Path("/")`,
		URL:      e.URL,
		Protocol: engine.HTTPS,
		KeyPair:  &engine.KeyPair{Key: localhostKey, Cert: localhostCert},
	})

	c.Assert(s.mux.UpsertHost(b.H), IsNil)
	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "hi https")
	// Make sure that we see right proto
	c.Assert(req.Header.Get("X-Forwarded-Proto"), Equals, "https")
}

func (s *ServerSuite) TestServerUpdateHTTPS(c *C) {
	e := testutils.NewHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hi https"))
	})
	defer e.Close()

	b := MakeBatch(Batch{
		Addr:     "localhost:41000",
		Route:    `Path("/")`,
		URL:      e.URL,
		Protocol: engine.HTTPS,
		KeyPair:  &engine.KeyPair{Key: localhostKey, Cert: localhostCert},
	})
	b.L.Settings = &engine.HTTPSListenerSettings{TLS: engine.TLSSettings{MinVersion: "VersionTLS11"}}
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	config := &tls.Config{
		InsecureSkipVerify: true,
		// We only support tls 10
		MinVersion: tls.VersionTLS10,
		MaxVersion: tls.VersionTLS10,
	}

	conn, err := tls.Dial("tcp", b.L.Address.Address, config)
	c.Assert(err, NotNil) // we got TLS error

	// Relax the version
	b.L.Settings = &engine.HTTPSListenerSettings{TLS: engine.TLSSettings{MinVersion: "VersionTLS10"}}
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	time.Sleep(20 * time.Millisecond)

	conn, err = tls.Dial("tcp", b.L.Address.Address, config)
	c.Assert(err, IsNil)

	fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')

	c.Assert(status, Equals, "HTTP/1.0 200 OK\r\n")
	state := conn.ConnectionState()
	c.Assert(state.Version, DeepEquals, uint16(tls.VersionTLS10))
	conn.Close()
}

func (s *ServerSuite) TestBackendHTTPS(c *C) {
	e := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hi https"))
		}))
	e.StartTLS()
	defer e.Close()

	b := MakeBatch(Batch{
		Addr:  "localhost:41000",
		Route: `Path("/")`,
		URL:   e.URL,
	})
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	re, _, err := testutils.Get(b.FrontendURL("/"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, 500) // failed because of bad cert

	b.B.Settings = engine.HTTPBackendSettings{TLS: &engine.TLSSettings{InsecureSkipVerify: true}}
	c.Assert(s.mux.UpsertBackend(b.B), IsNil)

	re, body, err := testutils.Get(b.FrontendURL("/"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, 200)
	c.Assert(string(body), Equals, "hi https")
}

func (s *ServerSuite) TestHostKeyPairUpdate(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{
		Addr:     "localhost:31001",
		Route:    `Path("/")`,
		URL:      e.URL,
		Protocol: engine.HTTPS,
		KeyPair:  &engine.KeyPair{Key: localhostKey, Cert: localhostCert},
	})
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
	certserial1 := getPeerCertSerialNo(c, b.FrontendURL("/"))

	b.H.Settings.KeyPair = &engine.KeyPair{Key: otherHostKey, Cert: otherHostCert}

	c.Assert(s.mux.UpsertHost(b.H), IsNil)
	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
	certserial2 := getPeerCertSerialNo(c, b.FrontendURL("/"))

	//Ensure different certs were returned
	c.Assert(certserial1, Not(Equals), certserial2)
}

func (s *ServerSuite) TestOCSPStapling(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()
	srv := NewOCSPResponder()
	defer srv.Close()

	b := MakeBatch(Batch{
		Addr:     "localhost:31000",
		Route:    `Path("/")`,
		URL:      e.URL,
		Protocol: engine.HTTPS,
	})
	b.H.Settings = engine.HostSettings{
		KeyPair: &engine.KeyPair{Key: LocalhostKey, Cert: LocalhostCertChain},
		OCSP:    engine.OCSPSettings{Enabled: true, Period: "1h", Responders: []string{srv.URL}, SkipSignatureCheck: true},
	}
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	conn, err := tls.Dial("tcp", b.L.Address.Address, &tls.Config{
		InsecureSkipVerify: true,
	})

	c.Assert(err, IsNil)
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\n\r\n")
	re := conn.OCSPResponse()
	c.Assert(re, DeepEquals, OCSPResponseBytes)

	conn.Close()

	// Make sure that deleting the host clears the cache
	hk := engine.HostKey{Name: b.H.Name}
	c.Assert(s.mux.stapler.HasHost(hk), Equals, true)
	c.Assert(s.mux.DeleteHost(hk), IsNil)
	c.Assert(s.mux.stapler.HasHost(hk), Equals, false)
}

func (s *ServerSuite) TestOCSPResponderDown(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	srv := NewOCSPResponder()
	srv.Close()

	b := MakeBatch(Batch{
		Addr:     "localhost:31000",
		Route:    `Path("/")`,
		URL:      e.URL,
		Protocol: engine.HTTPS,
	})
	b.H.Settings = engine.HostSettings{
		KeyPair: &engine.KeyPair{Key: LocalhostKey, Cert: LocalhostCertChain},
		OCSP:    engine.OCSPSettings{Enabled: true, Period: "1h", Responders: []string{srv.URL}, SkipSignatureCheck: true},
	}
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	conn, err := tls.Dial("tcp", b.L.Address.Address, &tls.Config{
		InsecureSkipVerify: true,
	})

	c.Assert(err, IsNil)
	fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	c.Assert(err, IsNil)

	c.Assert(status, Equals, "HTTP/1.0 200 OK\r\n")
	re := conn.OCSPResponse()
	c.Assert(re, IsNil)
	conn.Close()
}

func (s *ServerSuite) TestSNI(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint 1")
	defer e.Close()

	e2 := testutils.NewResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	b := MakeBatch(Batch{
		Host:     "localhost",
		Addr:     "localhost:41000",
		Route:    `Path("/path1")`,
		URL:      e.URL,
		Protocol: engine.HTTPS,
		KeyPair:  &engine.KeyPair{Key: localhostKey, Cert: localhostCert},
	})
	b2 := MakeBatch(Batch{
		Host:     "otherhost",
		Addr:     "localhost:41000",
		Route:    `Path("/path2")`,
		URL:      e2.URL,
		Protocol: engine.HTTPS,
		KeyPair:  &engine.KeyPair{Key: otherHostKey, Cert: otherHostCert},
	})
	b2.H.Settings.Default = true
	c.Assert(s.mux.Init(MakeSnapshot(b, b2)), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	//For the same path, if the Hostname is different (SNI), then return a different Cert - true host differentiation.
	c.Assert(getPeerCertSerialNo(c, b.FrontendURL("/path1"), testutils.Host("example.com")), Equals, "77bdc3e97d00584f03faec7cda682cf")
	c.Assert(getPeerCertSerialNo(c, b.FrontendURL("/path1"), testutils.Host("otherhost")), Equals, "c3244866e57c7b1f")

	//For a non-specified host, return default Cert
	c.Assert(getPeerCertSerialNo(c, b.FrontendURL("/path1"), testutils.Host("non-example.com")), Equals, "c3244866e57c7b1f")
}

func (s *ServerSuite) TestMiddlewareCRUD(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint 1")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e.URL,
	})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	// 1 request per second
	rl := MakeRateLimit(UID("rl"), 1, "client.ip", 1, 1)

	_, err := rl.Middleware.NewHandler(nil)
	c.Assert(err, IsNil)

	c.Assert(s.mux.UpsertMiddleware(b.FK, rl), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
	re, _, err := testutils.Get(MakeURL(b.L, "/"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, 429) // too many requests

	c.Assert(s.mux.DeleteMiddleware(engine.MiddlewareKey{FrontendKey: b.FK, Id: rl.Id}), IsNil)
	for i := 0; i < 3; i++ {
		c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
		c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
	}
}

func (s *ServerSuite) TestMiddlewareOrder(c *C) {
	var req *http.Request
	e := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		req = r
		w.Write([]byte("done"))
	})
	defer e.Close()

	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e.URL,
	})
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	a1 := engine.Middleware{
		Priority:   0,
		Type:       "appender",
		Id:         "a1",
		Middleware: &appender{append: "a1"},
	}

	a2 := engine.Middleware{
		Priority:   1,
		Type:       "appender",
		Id:         "a0",
		Middleware: &appender{append: "a2"},
	}

	c.Assert(s.mux.UpsertMiddleware(b.FK, a1), IsNil)
	c.Assert(s.mux.UpsertMiddleware(b.FK, a2), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "done")
	c.Assert(req.Header["X-Append"], DeepEquals, []string{"a1", "a2"})
}

func (s *ServerSuite) TestMiddlewareUpdate(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint 1")
	defer e.Close()

	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e.URL,
	})
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	// 1 request per second
	rl := MakeRateLimit(UID("rl"), 1, "client.ip", 1, 1)

	_, err := rl.Middleware.NewHandler(nil)
	c.Assert(err, IsNil)

	c.Assert(s.mux.UpsertMiddleware(b.FK, rl), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
	re, _, err := testutils.Get(MakeURL(b.L, "/"))
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, 429) // too many requests

	// 100 requests per second
	rl = MakeRateLimit(rl.Id, 100, "client.ip", 100, 1)

	c.Assert(s.mux.UpsertMiddleware(b.FK, rl), IsNil)

	for i := 0; i < 3; i++ {
		c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
		c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
	}
}

func (s *ServerSuite) TestFrontendOptionsCRUD(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint 1")
	defer e.Close()

	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e.URL,
	})
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	body := "Hello, this request is longer than 8 bytes"
	response, bodyBytes, err := testutils.MakeRequest(MakeURL(b.L, "/"), testutils.Body(body))
	c.Assert(err, IsNil)
	c.Assert(string(bodyBytes), Equals, "Hi, I'm endpoint 1")
	c.Assert(response.StatusCode, Equals, http.StatusOK)

	settings := engine.HTTPFrontendSettings{
		Limits: engine.HTTPFrontendLimits{
			MaxBodyBytes: 8,
		},
		FailoverPredicate: "IsNetworkError()",
	}
	b.F.Settings = settings

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)

	response, _, err = testutils.MakeRequest(MakeURL(b.L, "/"), testutils.Body(body))
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusRequestEntityTooLarge)
}

func (s *ServerSuite) TestFrontendSwitchBackend(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	e1 := testutils.NewResponder("1")
	defer e1.Close()

	e2 := testutils.NewResponder("2")
	defer e2.Close()

	e3 := testutils.NewResponder("3")
	defer e3.Close()

	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e1.URL,
	})

	s1, s2, s3 := MakeServer(e1.URL), MakeServer(e2.URL), MakeServer(e3.URL)

	c.Assert(s.mux.UpsertServer(b.BK, s1), IsNil)
	c.Assert(s.mux.UpsertServer(b.BK, s2), IsNil)

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	b2 := MakeBackend()
	b2k := engine.BackendKey{Id: b2.Id}
	c.Assert(s.mux.UpsertServer(b2k, s2), IsNil)
	c.Assert(s.mux.UpsertServer(b2k, s3), IsNil)

	responseSet := make(map[string]bool)
	responseSet[GETResponse(c, b.FrontendURL("/"))] = true
	responseSet[GETResponse(c, b.FrontendURL("/"))] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true, "2": true})

	b.F.BackendId = b2k.Id
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)

	responseSet = make(map[string]bool)
	responseSet[GETResponse(c, b.FrontendURL("/"))] = true
	responseSet[GETResponse(c, b.FrontendURL("/"))] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"2": true, "3": true})
}

func (s *ServerSuite) TestFrontendUpdateRoute(c *C) {
	e := testutils.NewResponder("hola")
	defer e.Close()

	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e.URL,
	})
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "hola")

	b.F.Route = `Path("/New")`

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(GETResponse(c, b.FrontendURL("/New")), Equals, "hola")

	response, _, err := testutils.Get(MakeURL(b.L, "/"))
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusNotFound)
}

func (s *ServerSuite) TestFrontendRestoreRoute(c *C) {
	e := testutils.NewResponder("onestraw")
	defer e.Close()

	log.Infof("testing onestraw's patch")
	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e.URL,
	})
	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
	c.Assert(s.mux.Start(), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "onestraw")

	b.F.Route = `Path("/New")`

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(GETResponse(c, b.FrontendURL("/New")), Equals, "onestraw")

	b.F.Route = `Path("/")`

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "onestraw")

	response, _, err := testutils.Get(MakeURL(b.L, "/New"))
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusNotFound)
}

func (s *ServerSuite) TestBackendUpdate(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	e1 := testutils.NewResponder("1")
	defer e1.Close()

	e2 := testutils.NewResponder("2")
	defer e2.Close()

	b := MakeBatch(Batch{
		Addr:  "localhost:31000",
		Route: `Path("/")`,
		URL:   e1.URL,
	})

	s1, s2 := MakeServer(e1.URL), MakeServer(e2.URL)

	c.Assert(s.mux.UpsertServer(b.BK, s1), IsNil)
	c.Assert(s.mux.UpsertServer(b.BK, s2), IsNil)

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	responseSet := make(map[string]bool)
	responseSet[GETResponse(c, b.FrontendURL("/"))] = true
	responseSet[GETResponse(c, b.FrontendURL("/"))] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true, "2": true})

	sk2 := engine.ServerKey{BackendKey: b.BK, Id: s2.Id}
	c.Assert(s.mux.DeleteServer(sk2), IsNil)

	responseSet = make(map[string]bool)
	responseSet[GETResponse(c, b.FrontendURL("/"))] = true
	responseSet[GETResponse(c, b.FrontendURL("/"))] = true

	c.Assert(responseSet, DeepEquals, map[string]bool{"1": true})
}

func (s *ServerSuite) TestServerAddBad(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{Addr: "localhost:11500", Route: `Path("/")`, URL: e.URL})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")

	bad := engine.Server{Id: UID("srv"), URL: ""}
	c.Assert(s.mux.UpsertServer(b.BK, bad), NotNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func (s *ServerSuite) TestServerUpsertURL(c *C) {
	c.Assert(s.mux.Start(), IsNil)

	e1 := testutils.NewResponder("Hi, I'm endpoint 1")
	defer e1.Close()

	e2 := testutils.NewResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	b := MakeBatch(Batch{Addr: "localhost:11300", Route: `Path("/")`, URL: e1.URL})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")

	b.S.URL = e2.URL
	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 2")
}

func (s *ServerSuite) TestBackendUpdateOptions(c *C) {
	e := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("slow server"))
	})
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{Addr: "localhost:11300", Route: `Path("/")`, URL: e.URL})

	settings := b.B.HTTPSettings()
	settings.Timeouts = engine.HTTPBackendTimeouts{Read: "1ms"}
	b.B.Settings = settings

	c.Assert(s.mux.UpsertBackend(b.B), IsNil)
	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	re, _, err := testutils.Get(MakeURL(b.L, "/"))
	c.Assert(err, IsNil)
	c.Assert(re, NotNil)
	c.Assert(re.StatusCode, Equals, http.StatusGatewayTimeout)

	settings.Timeouts = engine.HTTPBackendTimeouts{Read: "20ms"}
	b.B.Settings = settings

	c.Assert(s.mux.UpsertBackend(b.B), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "slow server")
}

func (s *ServerSuite) TestSwitchBackendOptions(c *C) {
	e := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("slow server"))
	})
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{Addr: "localhost:11300", Route: `Path("/")`, URL: e.URL})

	settings := b.B.HTTPSettings()
	settings.Timeouts = engine.HTTPBackendTimeouts{Read: "1ms"}
	b.B.Settings = settings

	b2 := MakeBackend()
	settings = b2.HTTPSettings()
	settings.Timeouts = engine.HTTPBackendTimeouts{Read: "20ms"}
	b2.Settings = settings

	c.Assert(s.mux.UpsertBackend(b.B), IsNil)
	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)

	c.Assert(s.mux.UpsertBackend(b2), IsNil)
	c.Assert(s.mux.UpsertServer(engine.BackendKey{Id: b2.Id}, b.S), IsNil)

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	re, _, err := testutils.Get(MakeURL(b.L, "/"))
	c.Assert(err, IsNil)
	c.Assert(re, NotNil)
	c.Assert(re.StatusCode, Equals, http.StatusGatewayTimeout)

	b.F.BackendId = b2.Id
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "slow server")
}

func (s *ServerSuite) TestFilesNoFiles(c *C) {
	files, err := s.mux.GetFiles()
	c.Assert(err, IsNil)
	c.Assert(len(files), Equals, 0)
	c.Assert(s.mux.Start(), IsNil)
}

func (s *ServerSuite) TestTakeFiles(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint 1")
	defer e.Close()

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{
		Addr:     "localhost:41000",
		Route:    `Path("/")`,
		URL:      e.URL,
		Protocol: engine.HTTPS,
		KeyPair:  &engine.KeyPair{Key: localhostKey, Cert: localhostCert},
	})

	c.Assert(s.mux.UpsertHost(b.H), IsNil)
	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)
	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")

	mux2, err := New(s.lastId, s.st, proxy.Options{})
	c.Assert(err, IsNil)

	e2 := testutils.NewResponder("Hi, I'm endpoint 2")
	defer e2.Close()

	b2 := MakeBatch(Batch{
		Addr:     "localhost:41000",
		Route:    `Path("/")`,
		URL:      e2.URL,
		Protocol: engine.HTTPS,
		KeyPair:  &engine.KeyPair{Key: otherHostKey, Cert: otherHostCert},
	})

	c.Assert(mux2.UpsertHost(b2.H), IsNil)
	c.Assert(mux2.UpsertServer(b2.BK, b2.S), IsNil)
	c.Assert(mux2.UpsertFrontend(b2.F), IsNil)
	c.Assert(mux2.UpsertListener(b2.L), IsNil)

	files, err := s.mux.GetFiles()
	c.Assert(err, IsNil)
	c.Assert(mux2.TakeFiles(files), IsNil)

	c.Assert(mux2.Start(), IsNil)
	s.mux.Stop(true)
	defer mux2.Stop(true)

	c.Assert(GETResponse(c, b2.FrontendURL("/")), Equals, "Hi, I'm endpoint 2")
}

// TODO(thrawn01): rtmcollect is disabled due to memory leak
// Server RTM metrics are not affected by upserts.
//func (s *ServerSuite) TestSrvRTMOnUpsert(c *C) {
//	e1 := testutils.NewResponder("Hi, I'm endpoint 1")
//	defer e1.Close()
//
//	b := MakeBatch(Batch{Addr: "localhost:11300", Route: `Path("/")`, URL: e1.URL})
//	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
//	c.Assert(s.mux.Start(), IsNil)
//	defer s.mux.Stop(true)
//
//	// When: an existing backend server upserted during operation
//	for i := 0; i < 3; i++ {
//		c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
//	}
//	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
//	for i := 0; i < 4; i++ {
//		c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
//	}
//
//	// Then: total count includes metrics collected before and after an upsert.
//	rts, err := s.mux.ServerStats(b.SK)
//	c.Assert(err, IsNil)
//	c.Assert(rts.Counters.Total, Equals, int64(7))
//}

// TODO(thrawn01): rtmcollect is disabled due to memory leak
// Server RTM metrics are not affected by upserts.
//func (s *ServerSuite) TestSrvRTMOnDelete(c *C) {
//	e1 := testutils.NewResponder("Hi, I'm endpoint 1")
//	defer e1.Close()
//
//	b := MakeBatch(Batch{Addr: "localhost:11300", Route: `Path("/")`, URL: e1.URL})
//	c.Assert(s.mux.Init(b.Snapshot()), IsNil)
//	c.Assert(s.mux.Start(), IsNil)
//	defer s.mux.Stop(true)
//
//	// When: an existing backend server is removed and added again.
//	for i := 0; i < 3; i++ {
//		c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
//	}
//	c.Assert(s.mux.DeleteServer(b.SK), IsNil)
//	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)
//	for i := 0; i < 4; i++ {
//		c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint 1")
//	}
//
//	// Then: total count includes only metrics after the server was re-added.
//	rts, err := s.mux.ServerStats(b.SK)
//	c.Assert(err, IsNil)
//	c.Assert(rts.Counters.Total, Equals, int64(4))
//}

// TODO(thrawn01): rtmcollect is disabled due to memory leak
//func (s *ServerSuite) TestGetStats(c *C) {
//	e1 := testutils.NewResponder("Hi, I'm endpoint 1")
//	defer e1.Close()
//	e2 := testutils.NewResponder("Hi, I'm endpoint 2")
//	defer e2.Close()
//
//	beCfg := MakeBackend()
//	c.Assert(s.mux.UpsertBackend(beCfg), IsNil)
//	beSrvCfg1 := MakeServer(e1.URL)
//	c.Assert(s.mux.UpsertServer(beCfg.Key(), beSrvCfg1), IsNil)
//	beSrvCfg2 := MakeServer(e2.URL)
//	c.Assert(s.mux.UpsertServer(beCfg.Key(), beSrvCfg2), IsNil)
//
//	liCfg := MakeListener("localhost:11300", engine.HTTP)
//	c.Assert(s.mux.UpsertListener(liCfg), IsNil)
//	feCfg1 := MakeFrontend(`Path("/foo")`, beCfg.GetId())
//	c.Assert(s.mux.UpsertFrontend(feCfg1), IsNil)
//	feCfg2 := MakeFrontend(`Path("/bar")`, beCfg.GetId())
//	c.Assert(s.mux.UpsertFrontend(feCfg2), IsNil)
//
//	c.Assert(s.mux.Start(), IsNil)
//	defer s.mux.Stop(true)
//
//	for i := 0; i < 10; i++ {
//		GETResponse(c, MakeURL(liCfg, "/foo"))
//	}
//
//	stats, err := s.mux.ServerStats(engine.ServerKey{BackendKey: beCfg.Key(), Id: beSrvCfg1.GetId()})
//	c.Assert(err, IsNil)
//	c.Assert(stats, NotNil)
//
//	feStats1, err := s.mux.FrontendStats(feCfg1.Key())
//	c.Assert(feStats1, NotNil)
//	c.Assert(err, IsNil)
//
//	feStats2, err := s.mux.FrontendStats(feCfg2.Key())
//	c.Assert(feStats2, IsNil)
//	c.Assert(err.Error(), Matches, "frontend frontend\\d+ RT not collected")
//
//	bStats, err := s.mux.BackendStats(beCfg.Key())
//	c.Assert(bStats, NotNil)
//	c.Assert(err, IsNil)
//
//	topF, err := s.mux.TopFrontends(nil)
//	c.Assert(err, IsNil)
//	c.Assert(len(topF), Equals, 1)
//
//	topServers, err := s.mux.TopServers(nil)
//	c.Assert(err, IsNil)
//	c.Assert(len(topServers), Equals, 2)
//
//	// emit stats works without errors
//	c.Assert(s.mux.emitMetrics(), IsNil)
//}

// If there is no such frontend registered in the multiplexer then
// 404 Not Found is returned.
func (s *ServerSuite) TestNotFound(c *C) {
	c.Assert(s.mux.Start(), IsNil)
	defer s.mux.Stop(true)
	beCfg := MakeBackend()
	c.Assert(s.mux.UpsertBackend(beCfg), IsNil)
	liCfg := MakeListener("localhost:11300", engine.HTTP)
	c.Assert(s.mux.UpsertListener(liCfg), IsNil)

	// When
	rs, msg, err := testutils.Get("http://localhost:11300/foo")

	// Then
	c.Assert(err, IsNil)
	c.Assert(rs.StatusCode, Equals, http.StatusNotFound)
	c.Assert(string(msg), Equals, `{"error":"not found"}`)
}

func (s *ServerSuite) TestNoBackendServers(c *C) {
	c.Assert(s.mux.Start(), IsNil)
	defer s.mux.Stop(true)
	beCfg := MakeBackend()
	c.Assert(s.mux.UpsertBackend(beCfg), IsNil)
	liCfg := MakeListener("localhost:11300", engine.HTTP)
	c.Assert(s.mux.UpsertListener(liCfg), IsNil)
	feCfg := MakeFrontend(`Path("/foo")`, beCfg.GetId())
	c.Assert(s.mux.UpsertFrontend(feCfg), IsNil)

	// When
	rs, _, err := testutils.Get("http://localhost:11300/foo")

	// Then
	c.Assert(err, IsNil)
	c.Assert(rs.StatusCode, Equals, http.StatusServiceUnavailable)
}

func (s *ServerSuite) TestCustomNotFound(c *C) {
	st := stapler.New()
	m, err := New(s.lastId, st, proxy.Options{NotFoundMiddleware: &appender{append: "Custom Not Found handler"}})
	c.Assert(err, IsNil)
	t := reflect.TypeOf(m.router.GetNotFound())
	c.Assert(t.String(), Equals, "*mux.appender")
}

// X-Forward-(For|Proto|Host) headers are either overwritten or augmented
// depending on the TrustFWDH config of the frontend and multiplexer.
func TestProxyHeaders(t *testing.T) {
	var rq *http.Request
	tbs := testutils.NewHandler(func(w http.ResponseWriter, r *http.Request) {
		rq = r
	})
	defer tbs.Close()

	for _, tc := range []struct {
		name         string
		feTrustFXDH  bool
		muxTrustFXDH bool
		xfdFor       []string
		xfdProto     []string
		xfdHost      []string
	}{{
		name:         "Test case 0",
		feTrustFXDH:  false,
		muxTrustFXDH: false,
		xfdFor:       []string{"127.0.0.1"},
		xfdProto:     []string{"http"},
		// thrawn01: Previously, this test asserted the forward host was the same as the `tbs` address,
		// which was incorrect in old version of oxy. Now it is correctly set as `Host` header
		// as given to mux when client makes the request.
		xfdHost: []string{"127.0.0.1:11300"},
	}, {
		name:         "Test case 1",
		feTrustFXDH:  true,
		muxTrustFXDH: false,
		xfdFor:       []string{"a, b, c, 127.0.0.1"},
		xfdProto:     []string{"d, e", "f"},
		xfdHost:      []string{"g, h", "i"},
	}, {
		name:         "Test case 2",
		feTrustFXDH:  false,
		muxTrustFXDH: true,
		xfdFor:       []string{"a, b, c, 127.0.0.1"},
		xfdProto:     []string{"d, e", "f"},
		xfdHost:      []string{"g, h", "i"},
	}} {

		t.Run(tc.name, func(t *testing.T) {
			var err error
			mux, err := New(0, stapler.New(), proxy.Options{TrustForwardHeader: tc.muxTrustFXDH})
			assert.NoError(t, err)
			// We have to start stop multiplexer for every case to ensure that
			// the frontend is initialized on each iteration.
			assert.Nil(t, mux.Start())

			beCfg := MakeBackend()
			assert.NoError(t, mux.UpsertBackend(beCfg))
			beSrvCfg := MakeServer(tbs.URL)
			assert.NoError(t, mux.UpsertServer(beCfg.Key(), beSrvCfg))
			liCfg := MakeListener("127.0.0.1:11300", engine.HTTP)
			assert.NoError(t, mux.UpsertListener(liCfg))
			feCfg := MakeFrontend(`Path("/")`, beCfg.GetId())

			var httpSettings engine.HTTPFrontendSettings
			httpSettings.TrustForwardHeader = tc.feTrustFXDH
			feCfg.Settings = httpSettings
			assert.NoError(t, mux.UpsertFrontend(feCfg))

			// When
			rs, _, err := testutils.Get("http://127.0.0.1:11300/",
				testutils.Header("X-Forwarded-For", "a, b"),
				testutils.Header("X-Forwarded-For", "c"),
				testutils.Header("X-Forwarded-Proto", "d, e"),
				testutils.Header("X-Forwarded-Proto", "f"),
				testutils.Header("X-Forwarded-Host", "g, h"),
				testutils.Header("X-Forwarded-Host", "i"))

			// Then
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rs.StatusCode)
			assert.Equal(t, tc.xfdFor, rq.Header["X-Forwarded-For"])
			assert.Equal(t, tc.xfdProto, rq.Header["X-Forwarded-Proto"])
			assert.Equal(t, tc.xfdHost, rq.Header["X-Forwarded-Host"])

			mux.Stop(true)
		})
	}
}

// Test case when user defined a localhost alias
func (s *ServerSuite) TestAliases(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint 1")
	defer e.Close()

	// New proxy with an alias that replaces "localhost" with "alias"
	mux, err := New(s.lastId, s.st, proxy.Options{
		Aliases: map[string]string{`Host("localhost")`: `Host("alias")`},
	})

	c.Assert(err, IsNil)
	s.mux = mux

	c.Assert(s.mux.Start(), IsNil)

	b := MakeBatch(Batch{Addr: "localhost:41000", Route: `Host("localhost") && Path("/")`, URL: e.URL})

	c.Assert(s.mux.UpsertServer(b.BK, b.S), IsNil)

	c.Assert(s.mux.UpsertFrontend(b.F), IsNil)

	c.Assert(s.mux.UpsertListener(b.L), IsNil)

	c.Assert(GETResponse(c, b.FrontendURL("/"), testutils.Host("localhost")), Equals, "Hi, I'm endpoint 1")
	c.Assert(GETResponse(c, b.FrontendURL("/"), testutils.Host("alias")), Equals, "Hi, I'm endpoint 1")
}

func GETResponse(c *C, url string, opts ...testutils.ReqOption) string {
	response, body, err := testutils.Get(url, opts...)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	return string(body)
}

func getResponse(t *testing.T, url string, opts ...testutils.ReqOption) string {
	t.Helper()
	response, body, err := testutils.Get(url, opts...)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	return string(body)
}

func getPeerCertSerialNo(c *C, url string, opts ...testutils.ReqOption) string {
	response, _, err := testutils.Get(url, opts...)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	c.Assert(response.TLS, NotNil)
	return response.TLS.PeerCertificates[0].SerialNumber.Text(16)
}

// localhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at the last second of 2049 (the end
// of ASN.1 time).
// generated from src/pkg/crypto/tls:
// go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBjjCCATigAwIBAgIQB3vcPpfQBYTwP67HzaaCzzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJB
AMh0FPD04nXvhk1VygciBIk6C3wgsCEECBoQ4HP4A+6Jby1K5Gr7k4CvGIzCKV+j
vJ5ZvYsFpvO8oeNSsma+SukCAwEAAaNoMGYwDgYDVR0PAQH/BAQDAgKkMBMGA1Ud
JQQMMAoGCCsGAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhh
bXBsZS5jb22HBH8AAAGHEAAAAAAAAAAAAAAAAAAAAAEwDQYJKoZIhvcNAQELBQAD
QQCORIV+fZpbzQmTh2YgrYxQVxfg/uAUbtC6CR0D/XYlIGMWeT7mWQtktc8XyR4s
c9IwOfyUgqQdBnWpYyGixiZz
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOQIBAAJBAMh0FPD04nXvhk1VygciBIk6C3wgsCEECBoQ4HP4A+6Jby1K5Gr7
k4CvGIzCKV+jvJ5ZvYsFpvO8oeNSsma+SukCAwEAAQJAHs54SW/ZPfbJ1SjSG7aG
q/BXw4PijbBo7liZpjj/obEH2cIDj1mdSiK7ZXfshzy3A5dTwDtFX0oTXHBIkgMk
AQIhAOYfq5f8Q5rP/ZN6SriKjDav0zbSQ6NK4L00Gu1iw/sRAiEA3v5THaFrzk0O
6evzoNdMBz7ip3hTQdC6EnkJsDeH4lkCICrISIaBB7CIaoQ4gBu+5kJkfcf7X0fE
a/PA9CCd9AGBAiBusElrlNvhfKihfsjhFt2bXyC8xmJ1cflbAA/KE9Z0iQIgWtLU
kHXBJ0l8y+3aFFuxpPuRZWUhOdAGpWgqjKSBWE8=
-----END RSA PRIVATE KEY-----`)

var otherHostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBWDCCAQKgAwIBAgIJAMMkSGblfHsfMA0GCSqGSIb3DQEBCwUAMBIxEDAOBgNV
BAMMB3Rlc3QtY2EwHhcNMTcxMDExMjAyNjE0WhcNMTcxMjEwMjAyNjE0WjAUMRIw
EAYDVQQDDAlvdGhlcmhvc3QwXDANBgkqhkiG9w0BAQEFAANLADBIAkEA3WS8QERs
4X8TnFceTnWJlSIxqxZR54Hvfn52zYv9pDkN/r4EZseTfpeWIFHHKUJvnPmZ8tDk
6t12c7LRzCvLdwIDAQABozkwNzAJBgNVHRMEAjAAMAsGA1UdDwQEAwIF4DAdBgNV
HSUEFjAUBggrBgEFBQcDAgYIKwYBBQUHAwEwDQYJKoZIhvcNAQELBQADQQA+wcWa
KINzfMA47U6ujZ62lfJfRUQ2R9WEbv0cH9jaq9AEH5UmMKaiyHpXWcUnKd8hN8bH
WjQsowgkBIB4kUjW
-----END CERTIFICATE-----`)

var otherHostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOQIBAAJBAN1kvEBEbOF/E5xXHk51iZUiMasWUeeB735+ds2L/aQ5Df6+BGbH
k36XliBRxylCb5z5mfLQ5OrddnOy0cwry3cCAwEAAQJAd5HHRiJud58NNVurx44d
X0kXcCJe29zGPxgIC902gLE6Y3FkD0forBqwTRwADFbT0eqfHHFEl1eK+C8CaMTo
0QIhAPiaeaI81JqGlZDQtyheyjL4qA3jcsKsyEOKmEvW6kP/AiEA4/sDBodHksOT
YxV3Nxu3DSdxh5yKDNu9RsJLFheSmIkCIF06iPzHdS9R40sAin9QNOGykEtNDZ9l
7mAt3HksaoP/AiAe+jeCBpWyGoMHXp5RTaHE1sw1Wg7kCmOgnrvnJ5LSyQIgTdFs
IwaQptPhBHhBeL0t/6gRNx+j1gnZP0hhYjH/7ZY=
-----END RSA PRIVATE KEY-----`)

var rsaIdKey = `-----BEGIN RSA PRIVATE KEY-----
MIIBOQIBAAJBAKieyY6ecPsJrvpWAkwyirR03f6WJbSDCWXbi56mLXoKXLLez3N7
X1CixTiQax3/yifDeT4Ou0+H/AqMnyhPHuUCAwEAAQJAL7BQ4uQOogEYGrbOiYxV
zDmtOz5txYK12rff4euvuu7ToQTqQ6XRochy7my3Ob/AnmVaCMnjVUAjeQOaGf3h
lQIhANrLKgmtSVZjQRwZzaKbWPGVIGveJBy+YSfKrXegus27AiEAxUtnxLPy8/Pp
znuJ+wNrKRfPUBVSDbjB0/GLYoLeq98CIAC5dX0strZzg66tIzIro4LBRKc2yBXU
R4wTLrnbrWKrAiAbBMOWNYqNDBc11sdDn+k5/G/AqNrO1EF/E/IhsIhsAwIgA7/g
2DlC1oLaH33zbVl69ldOfgaVJTafrfqq6lVr4vQ=
-----END RSA PRIVATE KEY-----`

var rsaIdKeyJwk = "eyJyZXNvdXJjZSI6Im5ldy1yZWcifQ"

var ecIdKey = `-----BEGIN EC PRIVATE KEY-----
MIGkAgEBBDDaF6eBwzSFXxRaPbTv4YS0AHwXhMye0iNh/DAyterab0/y7pHILBC4
rXUmliWtBEigBwYFK4EEACKhZANiAASlFMDW1zlF5QBWoZ99wdLBT/xjCmyjU8gs
AwI++wwHcTlMYasEV9n5CvoE6wvcjlpKa3MKgbK/qAWmH4Xc8h7vAuYNRFxwCpBt
Di7dbuoRmFYBqoGzLcFlZACaavu8xyI=
-----END EC PRIVATE KEY-----`

var ecIdKeyJwk = "eyJyZXNvdXJjZSI6Im5ldy1yZWcifQ"

type appender struct {
	next   http.Handler
	append string
}

func (a *appender) NewHandler(next http.Handler) (http.Handler, error) {
	return &appender{next: next, append: a.append}, nil
}

func (a *appender) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.Header.Add("X-Append", a.append)
	a.next.ServeHTTP(w, req)
}
