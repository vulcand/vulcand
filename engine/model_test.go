package engine

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"

	"github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/plugin/connlimit"
)

func TestBackend(t *testing.T) { TestingT(t) }

type BackendSuite struct {
}

var _ = Suite(&BackendSuite{})

func (s *BackendSuite) TestHostNew(c *C) {
	h, err := NewHost("localhost", HostOptions{})
	c.Assert(err, IsNil)
	c.Assert(h.Name, Equals, "localhost")
	c.Assert(h.Name, Equals, h.GetId())
	c.Assert(h.String(), Not(Equals), "")
}

func (s *BackendSuite) TestHostBad(c *C) {
	h, err := NewHost("", HostOptions{})
	c.Assert(err, NotNil)
	c.Assert(h, IsNil)
}

func (s *BackendSuite) TestFrontendDefaults(c *C) {
	f, err := NewHTTPFrontend("f1", "b1", HTTPFrontendSettings{Route: `Path("/home")`, Options: HTTPFrontendOptions{}})
	c.Assert(err, IsNil)
	c.Assert(f.GetId(), Equals, "f1")
	c.Assert(f.String(), Not(Equals), "")
	c.Assert(f.HTTPSettings().Route, Equals, `Path("/home")`)
}

func (s *BackendSuite) TestNewFrontendWithOptions(c *C) {
	options := HTTPFrontendOptions{
		Limits: HTTPFrontendLimits{
			MaxMemBodyBytes: 12,
			MaxBodyBytes:    400,
		},
		FailoverPredicate:  "IsNetworkError() && Attempts() <= 1",
		Hostname:           "host1",
		TrustForwardHeader: true,
	}
	settings := HTTPFrontendSettings{Route: `Path("/home")`, Options: options}
	f, err := NewHTTPFrontend("f1", "b1", settings)
	c.Assert(err, IsNil)
	c.Assert(f.Id, Equals, "f1")

	o, err := f.HTTPSettings().GetOptions()
	c.Assert(err, IsNil)

	c.Assert(o.Limits.MaxMemBodyBytes, Equals, int64(12))
	c.Assert(o.Limits.MaxBodyBytes, Equals, int64(400))

	c.Assert(o.FailoverPredicate, NotNil)
	c.Assert(o.TrustForwardHeader, Equals, true)
	c.Assert(o.Hostname, Equals, "host1")
}

func (s *BackendSuite) TestFrontendBadParams(c *C) {
	// Bad route
	_, err := NewHTTPFrontend("f1", "b1", HTTPFrontendSettings{Route: "/home  -- afawf \\~"})
	c.Assert(err, NotNil)

	// Empty params
	_, err = NewHTTPFrontend("", "", HTTPFrontendSettings{})
	c.Assert(err, NotNil)
}

func (s *BackendSuite) TestFrontendBadOptions(c *C) {
	options := []HTTPFrontendOptions{
		HTTPFrontendOptions{
			FailoverPredicate: "bad predicate",
		},
	}
	for _, o := range options {
		f, err := NewHTTPFrontend("f1", "b", HTTPFrontendSettings{Route: `Path("/home")`, Options: o})
		c.Assert(err, NotNil)
		c.Assert(f, IsNil)
	}
}

func (s *BackendSuite) TestBackendNew(c *C) {
	b, err := NewHTTPBackend("b1", HTTPBackendSettings{})
	c.Assert(err, IsNil)
	c.Assert(b.Type, Equals, HTTP)
	c.Assert(b.GetId(), Equals, "b1")
	c.Assert(b.String(), Not(Equals), "")
}

func (s *BackendSuite) TestNewBackendWithOptions(c *C) {
	options := HTTPBackendSettings{
		Timeouts: HTTPBackendTimeouts{
			Read:         "1s",
			Dial:         "2s",
			TLSHandshake: "3s",
		},
		KeepAlive: HTTPBackendKeepAlive{
			Period:              "4s",
			MaxIdleConnsPerHost: 3,
		},
	}
	b, err := NewHTTPBackend("b1", options)
	c.Assert(err, IsNil)
	c.Assert(b.GetId(), Equals, "b1")

	o, err := b.GetTransportOptions()
	c.Assert(err, IsNil)

	c.Assert(o.Timeouts.Read, Equals, time.Second)
	c.Assert(o.Timeouts.Dial, Equals, 2*time.Second)
	c.Assert(o.Timeouts.TlsHandshake, Equals, 3*time.Second)

	c.Assert(o.KeepAlive.Period, Equals, 4*time.Second)
	c.Assert(o.KeepAlive.MaxIdleConnsPerHost, Equals, 3)
}

func (s *BackendSuite) TestBackendOptionsEq(c *C) {
	options := []struct {
		a HTTPBackendSettings
		b HTTPBackendSettings
		e bool
	}{
		{HTTPBackendSettings{}, HTTPBackendSettings{}, true},

		{HTTPBackendSettings{Timeouts: HTTPBackendTimeouts{Dial: "1s"}}, HTTPBackendSettings{Timeouts: HTTPBackendTimeouts{Dial: "1s"}}, true},
		{HTTPBackendSettings{Timeouts: HTTPBackendTimeouts{Dial: "2s"}}, HTTPBackendSettings{Timeouts: HTTPBackendTimeouts{Dial: "1s"}}, false},
		{HTTPBackendSettings{Timeouts: HTTPBackendTimeouts{Read: "2s"}}, HTTPBackendSettings{Timeouts: HTTPBackendTimeouts{Read: "1s"}}, false},
		{HTTPBackendSettings{Timeouts: HTTPBackendTimeouts{TLSHandshake: "2s"}}, HTTPBackendSettings{Timeouts: HTTPBackendTimeouts{TLSHandshake: "1s"}}, false},

		{HTTPBackendSettings{KeepAlive: HTTPBackendKeepAlive{Period: "2s"}}, HTTPBackendSettings{KeepAlive: HTTPBackendKeepAlive{Period: "1s"}}, false},
		{HTTPBackendSettings{KeepAlive: HTTPBackendKeepAlive{MaxIdleConnsPerHost: 1}}, HTTPBackendSettings{KeepAlive: HTTPBackendKeepAlive{MaxIdleConnsPerHost: 2}}, false},
	}
	for _, o := range options {
		c.Assert(o.a.Equals(o.b), Equals, o.e)
	}
}

func (s *BackendSuite) TestNewBackendWithBadOptions(c *C) {
	options := []HTTPBackendSettings{
		HTTPBackendSettings{
			Timeouts: HTTPBackendTimeouts{
				Read: "1what?",
			},
		},
		HTTPBackendSettings{
			Timeouts: HTTPBackendTimeouts{
				Dial: "1what?",
			},
		},
		HTTPBackendSettings{
			Timeouts: HTTPBackendTimeouts{
				TLSHandshake: "1what?",
			},
		},
		HTTPBackendSettings{
			KeepAlive: HTTPBackendKeepAlive{
				Period: "1what?",
			},
		},
	}
	for _, o := range options {
		b, err := NewHTTPBackend("b1", o)
		c.Assert(err, NotNil)
		c.Assert(b, IsNil)
	}
}

func (s *BackendSuite) TestNewServer(c *C) {
	sv, err := NewServer("s1", "http://falhost")
	c.Assert(err, IsNil)
	c.Assert(sv.GetId(), Equals, "s1")
	c.Assert(sv.String(), Not(Equals), "")
}

func (s *BackendSuite) TestNewServerBadParams(c *C) {
	_, err := NewServer("s1", "http---")
	c.Assert(err, NotNil)
}

func (s *BackendSuite) TestFrontendsFromJSON(c *C) {
	f, err := NewHTTPFrontend("f1", "b1", HTTPFrontendSettings{Route: `Path("/path")`})
	c.Assert(err, IsNil)

	bytes, err := json.Marshal(f)

	fs := []Frontend{*f}

	bytes, err = json.Marshal(map[string]interface{}{"Frontends": fs})

	r := plugin.NewRegistry()
	c.Assert(r.AddSpec(connlimit.GetSpec()), IsNil)

	out, err := FrontendsFromJSON(bytes)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out, DeepEquals, fs)
}

func (s *BackendSuite) MiddlewareFromJSON(c *C) {
	cl, err := connlimit.NewConnLimit(10, "client.ip")
	c.Assert(err, IsNil)

	m := &Middleware{Id: "c1", Type: "connlimit", Middleware: cl}

	bytes, err := json.Marshal(m)
	c.Assert(err, IsNil)

	out, err := MiddlewareFromJSON(bytes, plugin.NewRegistry().GetSpec)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)
	c.Assert(out, DeepEquals, m)
}

func (s *BackendSuite) TestBackendFromJSON(c *C) {
	b, err := NewHTTPBackend("b1", HTTPBackendSettings{})
	c.Assert(err, IsNil)

	bytes, err := json.Marshal(b)
	c.Assert(err, IsNil)

	out, err := BackendFromJSON(bytes)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)

	c.Assert(out, DeepEquals, b)
}

func (s *BackendSuite) TestServerFromJSON(c *C) {
	e, err := NewServer("sv1", "http://localhost")
	c.Assert(err, IsNil)

	bytes, err := json.Marshal(e)
	c.Assert(err, IsNil)

	out, err := ServerFromJSON(bytes)
	c.Assert(err, IsNil)
	c.Assert(out, NotNil)

	c.Assert(out, DeepEquals, e)
}
