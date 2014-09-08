package supervisor

import (
	"testing"

	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestSupervisor(t *testing.T) { TestingT(t) }

type SupervisorSuite struct {
}

func (s *SupervisorSuite) SetUpTest(c *C) {
}

var _ = Suite(&SupervisorSuite{})
