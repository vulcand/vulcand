package registry

import (
	"testing"
	"time"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

type FakeRegistry struct {
	RegistrationCount int
}

func (s *FakeRegistry) RegisterApp(r *AppRegistration) error {
	s.RegistrationCount++
	return nil
}

func (s *FakeRegistry) RegisterHandler(r *HandlerRegistration) error {
	return nil
}

func TestRegistry(t *testing.T) {
	TestingT(t)
}

type RegistrySuite struct {
}

var _ = Suite(&RegistrySuite{})

func (s *RegistrySuite) TestStartRegistersAppAtInterval(c *C) {
	registration := &AppRegistration{}
	registry := &FakeRegistry{}

	heartbeater := NewHeartbeater(registration, registry, 10*time.Millisecond)
	heartbeater.Start()
	time.Sleep(30 * time.Millisecond)

	c.Assert(registry.RegistrationCount, Equals, 3)

	heartbeater.Stop()
	time.Sleep(30 * time.Millisecond)

	c.Assert(registry.RegistrationCount, Equals, 3)

	heartbeater.Start()
	time.Sleep(30 * time.Millisecond)

	c.Assert(registry.RegistrationCount, Equals, 6)
}
