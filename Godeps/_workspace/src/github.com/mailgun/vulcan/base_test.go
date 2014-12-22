/*
Declares gocheck's test suites
*/
package vulcan

import (
	. "gopkg.in/check.v1"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

//This is a simple suite to use if tests dont' need anything
//special
type MainSuite struct {
}

func (s *MainSuite) SetUpTest(c *C) {
}

var _ = Suite(&MainSuite{})
