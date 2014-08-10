package timetools

import (
	"time"
)

// TimeProvider is an interface we use to mock time in tests.
type TimeProvider interface {
	UtcNow() time.Time
	Sleep(time.Duration)
}

// RealTime is a real clock time, used in production.
type RealTime struct {
}

func (*RealTime) UtcNow() time.Time {
	return time.Now().UTC()
}

func (*RealTime) Sleep(d time.Duration) {
	time.Sleep(d)
}

// FreezedTime is manually controlled time for use in tests.
type FreezedTime struct {
	CurrentTime time.Time
}

func (t *FreezedTime) UtcNow() time.Time {
	return t.CurrentTime
}

func (t *FreezedTime) Sleep(d time.Duration) {
	t.CurrentTime = t.CurrentTime.Add(d)
}
