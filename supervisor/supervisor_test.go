package supervisor

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/testutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/backend/membackend"
	"github.com/mailgun/vulcand/plugin/registry"
	"github.com/mailgun/vulcand/server"
	. "github.com/mailgun/vulcand/testutils"
)

func TestSupervisor(t *testing.T) { TestingT(t) }

type SupervisorSuite struct {
	tm     *timetools.FreezedTime
	errorC chan error
	sv     *Supervisor
	b      *membackend.MemBackend
}

func (s *SupervisorSuite) SetUpTest(c *C) {

	s.b = membackend.NewMemBackend(registry.GetRegistry())

	s.errorC = make(chan error)

	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}

	s.sv = NewSupervisorWithOptions(newServer, s.b, s.errorC, Options{TimeProvider: s.tm})
}

func (s *SupervisorSuite) TearDownTest(c *C) {
	s.sv.Stop(true)
}

var _ = Suite(&SupervisorSuite{})

func (s *SupervisorSuite) TestStartStopEmpty(c *C) {
	s.sv.Start()
	fmt.Println("Stop")
}

func (s *SupervisorSuite) TestInitFromExistingConfig(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:33000", URL: e.URL})

	_, err := s.b.AddUpstream(l.Upstream)
	c.Assert(err, IsNil)

	_, err = s.b.AddHost(h)
	c.Assert(err, IsNil)

	_, err = s.b.AddLocation(l)
	c.Assert(err, IsNil)

	s.sv.Start()

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")
}

func (s *SupervisorSuite) TestInitOnTheFly(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	s.sv.Start()

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:33000", URL: e.URL})

	s.b.ChangesC <- &LocationAdded{
		Host:     h,
		Location: l,
	}

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")
}

func (s *SupervisorSuite) TestGracefulShutdown(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	s.sv.Start()

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:33000", URL: e.URL})

	s.b.ChangesC <- &LocationAdded{
		Host:     h,
		Location: l,
	}

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")
	close(s.b.ErrorsC)
}

func (s *SupervisorSuite) TestRestartOnBackendErrors(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:33000", URL: e.URL})

	_, err := s.b.AddUpstream(l.Upstream)
	c.Assert(err, IsNil)

	_, err = s.b.AddHost(h)
	c.Assert(err, IsNil)

	_, err = s.b.AddLocation(l)
	c.Assert(err, IsNil)

	s.sv.Start()

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")
	s.b.ErrorsC <- fmt.Errorf("restart")

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")
}

func (s *SupervisorSuite) TestTransferFiles(c *C) {
	e := NewTestResponder("Hi, I'm endpoint")
	defer e.Close()

	l, h := MakeLocation(LocOpts{Hostname: "localhost", Addr: "localhost:33000", URL: e.URL})

	_, err := s.b.AddUpstream(l.Upstream)
	c.Assert(err, IsNil)

	_, err = s.b.AddHost(h)
	c.Assert(err, IsNil)

	_, err = s.b.AddLocation(l)
	c.Assert(err, IsNil)

	s.sv.Start()

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")

	files, err := s.sv.GetFiles()
	c.Assert(err, IsNil)

	errorC := make(chan error)

	sv2 := NewSupervisorWithOptions(newServer, s.b, errorC, Options{TimeProvider: s.tm, Files: files})
	sv2.Start()
	s.sv.Stop(true)

	c.Assert(GETResponse(c, MakeURL(l, h.Listeners[0]), Opts{}), Equals, "Hi, I'm endpoint")
}

func GETResponse(c *C, url string, opts Opts) string {
	response, body, err := GET(url, opts)
	c.Assert(err, IsNil)
	c.Assert(response.StatusCode, Equals, http.StatusOK)
	return string(body)
}

func newServer(id int) (server.Server, error) {
	return server.NewMuxServerWithOptions(id, server.Options{})
}
