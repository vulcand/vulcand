package scroll

import (
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll/registry"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

type FakeRegistry struct {
	Registrations []*registry.HandlerRegistration
}

func (s *FakeRegistry) RegisterApp(*registry.AppRegistration) error {
	return nil
}

func (s *FakeRegistry) RegisterHandler(r *registry.HandlerRegistration) error {
	s.Registrations = append(s.Registrations, r)
	return nil
}

func TestApp(t *testing.T) {
	TestingT(t)
}

type AppSuite struct {
	config   AppConfig
	app      *App
	registry *FakeRegistry
}

var _ = Suite(&AppSuite{})

func (s *AppSuite) SetUpSuite(c *C) {
	s.registry = &FakeRegistry{}
	s.config = AppConfig{
		Name:             "test",
		ListenIP:         "0.0.0.0",
		ListenPort:       22000,
		Registry:         s.registry,
		Interval:         time.Millisecond,
		PublicAPIHost:    "public",
		ProtectedAPIHost: "protected",
	}
	s.app = NewAppWithConfig(s.config)
}

func (s *AppSuite) TestHeartbeaterLifecycle(c *C) {
	heartbeater := s.app.heartbeater

	go func() {
		s.app.Run()
	}()

	// Check that heartbeats start.
	time.Sleep(1 * time.Millisecond)
	c.Check(heartbeater.Running, Equals, true)

	// Check that heartbeats stops on SIGUSR1.
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Millisecond)
	c.Check(heartbeater.Running, Equals, false)

	// Check that heartbeater resumes on SIGUSR1.
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Millisecond)
	c.Check(heartbeater.Running, Equals, true)
}

func (s *AppSuite) TestRegistersHandler(c *C) {
	handlerSpec := Spec{
		Scopes:  []Scope{ScopePublic, ScopeProtected},
		Methods: []string{"GET"},
		Paths:   []string{"/"},
		Handler: index,
	}

	s.app.AddHandler(handlerSpec)

	public := s.registry.Registrations[0]
	protected := s.registry.Registrations[1]

	c.Check(public, DeepEquals, &registry.HandlerRegistration{
		Name:    "test",
		Host:    "public",
		Path:    "/",
		Methods: []string{"GET"},
	})
	c.Check(protected, DeepEquals, &registry.HandlerRegistration{
		Name:    "test",
		Host:    "protected",
		Path:    "/",
		Methods: []string{"GET"},
	})
}

func index(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	return Response{"message": "Hello World"}, nil
}
