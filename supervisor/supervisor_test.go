package supervisor

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/mailgun/timetools"
	"github.com/vulcand/oxy/testutils"
	"github.com/vulcand/vulcand/engine"
	"github.com/vulcand/vulcand/engine/memng"
	"github.com/vulcand/vulcand/plugin/registry"
	"github.com/vulcand/vulcand/proxy"
	"github.com/vulcand/vulcand/proxy/builder"
	"github.com/vulcand/vulcand/stapler"
	. "github.com/vulcand/vulcand/testutils"
	. "gopkg.in/check.v1"
)

func TestSupervisor(t *testing.T) { TestingT(t) }

func newProxy(id int) (proxy.Proxy, error) {
	return builder.NewProxy(id, stapler.New(), proxy.Options{})
}

type SupervisorSuite struct {
	clock *timetools.FreezedTime
	ng    *memng.Mem
}

func (s *SupervisorSuite) SetUpTest(c *C) {
	s.ng = memng.New(registry.GetRegistry()).(*memng.Mem)
	s.clock = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

var _ = Suite(&SupervisorSuite{})

func (s *SupervisorSuite) TestStartStopEmpty(c *C) {
	sup := New(newProxy, s.ng, Options{Clock: s.clock})
	err := sup.Start()
	c.Assert(err, IsNil)
	defer sup.Stop()
}

func (s *SupervisorSuite) TestInitFromExistingConfig(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:11800", Route: `Path("/")`, URL: e.URL})
	c.Assert(s.ng.UpsertBackend(b.B), IsNil)
	c.Assert(s.ng.UpsertServer(b.BK, b.S, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertFrontend(b.F, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertListener(b.L), IsNil)

	sup := New(newProxy, s.ng, Options{Clock: s.clock})

	// When
	c.Assert(sup.Start(), IsNil)
	defer sup.Stop()

	// Then
	time.Sleep(10 * time.Millisecond)
	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func (s *SupervisorSuite) TestInitOnTheFly(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	sup := New(newProxy, s.ng, Options{Clock: s.clock})
	err := sup.Start()
	c.Assert(err, IsNil)
	defer sup.Stop()

	// When
	b := MakeBatch(Batch{Addr: "localhost:11800", Route: `Path("/")`, URL: e.URL})
	c.Assert(s.ng.UpsertBackend(b.B), IsNil)
	c.Assert(s.ng.UpsertServer(b.BK, b.S, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertFrontend(b.F, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertListener(b.L), IsNil)

	// Then
	time.Sleep(10 * time.Millisecond)
	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func (s *SupervisorSuite) TestGracefulShutdown(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:11800", Route: `Path("/")`, URL: e.URL})
	c.Assert(s.ng.UpsertBackend(b.B), IsNil)
	c.Assert(s.ng.UpsertServer(b.BK, b.S, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertFrontend(b.F, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertListener(b.L), IsNil)

	sup := New(newProxy, s.ng, Options{Clock: s.clock})
	err := sup.Start()
	c.Assert(err, IsNil)
	defer sup.Stop()

	time.Sleep(10 * time.Millisecond)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
	close(s.ng.ErrorsC)
}

func (s *SupervisorSuite) TestRestartOnBackendErrors(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:11800", Route: `Path("/")`, URL: e.URL})
	c.Assert(s.ng.UpsertBackend(b.B), IsNil)
	c.Assert(s.ng.UpsertServer(b.BK, b.S, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertFrontend(b.F, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertListener(b.L), IsNil)

	sup := New(newProxy, s.ng, Options{Clock: s.clock})
	err := sup.Start()
	c.Assert(err, IsNil)
	defer sup.Stop()

	time.Sleep(10 * time.Millisecond)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
	s.ng.ErrorsC <- fmt.Errorf("restart")

	time.Sleep(10 * time.Millisecond)
	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func (s *SupervisorSuite) TestTransferFiles(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:11800", Route: `Path("/")`, URL: e.URL})
	c.Assert(s.ng.UpsertBackend(b.B), IsNil)
	c.Assert(s.ng.UpsertServer(b.BK, b.S, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertFrontend(b.F, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertListener(b.L), IsNil)

	sup := New(newProxy, s.ng, Options{Clock: s.clock})
	err := sup.Start()
	c.Assert(err, IsNil)

	time.Sleep(10 * time.Millisecond)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")

	files, err := sup.GetFiles()
	c.Assert(err, IsNil)

	sup2 := New(newProxy, s.ng, Options{Clock: s.clock, Files: files})
	err = sup2.Start()
	c.Assert(err, IsNil)
	defer sup2.Stop()

	sup.Stop()

	time.Sleep(10 * time.Millisecond)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func GETResponse(c *C, url string, opts ...testutils.ReqOption) string {
	response, body, err := testutils.Get(url, opts...)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	return string(body)
}
