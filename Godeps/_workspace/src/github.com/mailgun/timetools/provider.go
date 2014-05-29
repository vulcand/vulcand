package timetools

import (
	"time"
)

// This is the interface we use to mock time in tests
type TimeProvider interface {
	UtcNow() time.Time
}

//Real clock time, used in production
type RealTime struct {
}

func (*RealTime) UtcNow() time.Time {
	return time.Now().UTC()
}

// This is manually controlled time we use in tests
type FreezedTime struct {
	CurrentTime time.Time
}

func (t *FreezedTime) UtcNow() time.Time {
	return t.CurrentTime
}
