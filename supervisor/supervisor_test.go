package supervisor

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/oxy/testutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"github.com/mailgun/vulcand/engine"
	"github.com/mailgun/vulcand/engine/memng"
	"github.com/mailgun/vulcand/plugin/registry"
	"github.com/mailgun/vulcand/proxy"
	"github.com/mailgun/vulcand/stapler"
	. "github.com/mailgun/vulcand/testutils"
)

func TestSupervisor(t *testing.T) { TestingT(t) }

func newProxy(id int) (proxy.Proxy, error) {
	return proxy.New(id, stapler.New(), proxy.Options{})
}

type SupervisorSuite struct {
	clock  *timetools.FreezedTime
	errorC chan error
	sv     *Supervisor
	ng     *memng.Mem
}

func (s *SupervisorSuite) SetUpTest(c *C) {
	s.ng = memng.New(registry.GetRegistry()).(*memng.Mem)

	s.errorC = make(chan error)

	s.clock = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}

	s.sv = New(newProxy, s.ng, s.errorC, Options{Clock: s.clock})
}

func (s *SupervisorSuite) TearDownTest(c *C) {
	s.sv.Stop(true)
}

var _ = Suite(&SupervisorSuite{})

func (s *SupervisorSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})
}

func (s *SupervisorSuite) TestStartStopEmpty(c *C) {
	s.sv.Start()
	fmt.Println("Stop")
}

func (s *SupervisorSuite) TestInitFromExistingConfig(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	b := MakeBatch(Batch{Addr: "localhost:11800", Route: `Path("/")`, URL: e.URL})

	c.Assert(s.ng.UpsertBackend(b.B), IsNil)
	c.Assert(s.ng.UpsertServer(b.BK, b.S, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertFrontend(b.F, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertListener(b.L), IsNil)

	c.Assert(s.sv.Start(), IsNil)

	time.Sleep(10 * time.Millisecond)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func (s *SupervisorSuite) TestInitOnTheFly(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	s.sv.Start()

	b := MakeBatch(Batch{Addr: "localhost:11800", Route: `Path("/")`, URL: e.URL})

	c.Assert(s.ng.UpsertBackend(b.B), IsNil)
	c.Assert(s.ng.UpsertServer(b.BK, b.S, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertFrontend(b.F, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertListener(b.L), IsNil)

	time.Sleep(10 * time.Millisecond)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func (s *SupervisorSuite) TestGracefulShutdown(c *C) {
	e := testutils.NewResponder("Hi, I'm endpoint")
	defer e.Close()

	s.sv.Start()

	b := MakeBatch(Batch{Addr: "localhost:11800", Route: `Path("/")`, URL: e.URL})

	c.Assert(s.ng.UpsertBackend(b.B), IsNil)
	c.Assert(s.ng.UpsertServer(b.BK, b.S, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertFrontend(b.F, engine.NoTTL), IsNil)
	c.Assert(s.ng.UpsertListener(b.L), IsNil)

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

	s.sv.Start()

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

	s.sv.Start()

	time.Sleep(10 * time.Millisecond)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")

	files, err := s.sv.GetFiles()
	c.Assert(err, IsNil)

	errorC := make(chan error)

	sv2 := New(newProxy, s.ng, errorC, Options{Clock: s.clock, Files: files})
	sv2.Start()
	s.sv.Stop(true)

	time.Sleep(10 * time.Millisecond)

	c.Assert(GETResponse(c, b.FrontendURL("/")), Equals, "Hi, I'm endpoint")
}

func GETResponse(c *C, url string, opts ...testutils.ReqOption) string {
	response, body, err := testutils.Get(url, opts...)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	return string(body)
}
