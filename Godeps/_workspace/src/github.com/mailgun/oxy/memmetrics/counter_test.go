package memmetrics

import (
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

type CounterSuite struct {
	clock *timetools.FreezedTime
}

var _ = Suite(&CounterSuite{})

func (s *CounterSuite) SetUpSuite(c *C) {
	s.clock = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *CounterSuite) TestCloneExpired(c *C) {
	cnt, err := NewCounter(3, time.Second, CounterClock(s.clock))
	c.Assert(err, IsNil)
	cnt.Inc(1)
	s.clock.Sleep(time.Second)
	cnt.Inc(1)
	s.clock.Sleep(time.Second)
	cnt.Inc(1)
	s.clock.Sleep(time.Second)

	out := cnt.Clone()
	c.Assert(out.Count(), Equals, int64(2))
}
