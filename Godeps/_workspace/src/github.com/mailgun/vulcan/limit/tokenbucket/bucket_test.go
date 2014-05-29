package tokenbucket

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/launchpad.net/gocheck"
	"testing"
	"time"
)

func TestBucket(t *testing.T) { TestingT(t) }

type BucketSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&BucketSuite{})

func (s *BucketSuite) SetUpSuite(c *C) {
	s.tm = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

func (s *BucketSuite) TestConsumeSingleToken(c *C) {
	l, err := NewTokenBucket(Rate{1, time.Second}, 1, s.tm)
	c.Assert(err, Equals, nil)

	// First request passes
	delay, err := l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Next request does not pass the same second
	delay, err = l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Second)

	// Second later, the request passes
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Second)
	delay, err = l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Five seconds later, still only one request is allowed
	// because maxBurst is 1
	s.tm.CurrentTime = s.tm.CurrentTime.Add(5 * time.Second)
	delay, err = l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// The next one is forbidden
	delay, err = l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Second)
}

func (s *BucketSuite) TestFastConsumption(c *C) {
	l, err := NewTokenBucket(Rate{1, time.Second}, 1, s.tm)
	c.Assert(err, Equals, nil)

	// First request passes
	delay, err := l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Try 200 ms later
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Millisecond * 200)
	delay, err = l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Second)

	// Try 700 ms later
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Millisecond * 700)
	delay, err = l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Second)

	// Try 100 ms later, success!
	s.tm.CurrentTime = s.tm.CurrentTime.Add(time.Millisecond * 100)
	delay, err = l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))
}

func (s *BucketSuite) TestConsumeMultipleTokens(c *C) {
	l, err := NewTokenBucket(Rate{3, time.Second}, 5, s.tm)
	c.Assert(err, Equals, nil)

	delay, err := l.Consume(3)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	delay, err = l.Consume(2)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	delay, err = l.Consume(1)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Not(Equals), time.Duration(0))
}

func (s *BucketSuite) TestDelayIsCorrect(c *C) {
	l, err := NewTokenBucket(Rate{3, time.Second}, 5, s.tm)
	c.Assert(err, Equals, nil)

	// Exhaust initial capacity
	delay, err := l.Consume(5)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	delay, err = l.Consume(3)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Not(Equals), time.Duration(0))

	// Now wait provided delay and make sure we can consume now
	s.tm.CurrentTime = s.tm.CurrentTime.Add(delay)
	delay, err = l.Consume(3)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))
}

// Make sure requests that exceed burst size are not allowed
func (s *BucketSuite) TestExceedsBurst(c *C) {
	l, err := NewTokenBucket(Rate{1, time.Second}, 10, s.tm)
	c.Assert(err, Equals, nil)

	delay, err := l.Consume(11)
	c.Assert(err, Not(Equals), nil)
	c.Assert(delay, Equals, time.Duration(-1))
}

func (s *BucketSuite) TestConsumeBurst(c *C) {
	l, err := NewTokenBucket(Rate{2, time.Second}, 5, s.tm)
	c.Assert(err, Equals, nil)

	// In two seconds we would have 5 tokens
	s.tm.CurrentTime = s.tm.CurrentTime.Add(2 * time.Second)

	// Lets consume 5 at once
	delay, err := l.Consume(5)
	c.Assert(delay, Equals, time.Duration(0))
	c.Assert(err, Equals, nil)
}

func (s *BucketSuite) TestConsumeEstimate(c *C) {
	l, err := NewTokenBucket(Rate{2, time.Second}, 4, s.tm)
	c.Assert(err, Equals, nil)

	// Consume all burst at once
	delay, err := l.Consume(4)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(0))

	// Now try to consume it and face delay
	delay, err = l.Consume(4)
	c.Assert(err, Equals, nil)
	c.Assert(delay, Equals, time.Duration(2)*time.Second)
}

func (s *BucketSuite) TestInvalidParams(c *C) {
	// Invalid rate
	_, err := NewTokenBucket(Rate{0, 0}, 1, s.tm)
	c.Assert(err, Not(Equals), nil)

	// Invalid max tokens
	_, err = NewTokenBucket(Rate{2, time.Second}, 0, s.tm)
	c.Assert(err, Not(Equals), nil)

	// Invalid time provider
	_, err = NewTokenBucket(Rate{2, time.Second}, 1, nil)
	c.Assert(err, Not(Equals), nil)
}
