package timetools

import (
	"fmt"
	"time"
	"testing"
)

var _ = fmt.Printf // for testing

func TestRealTimeUtcNow(t *testing.T) {
	rt := RealTime{}

	rtNow := rt.UtcNow()
	atNow := time.Now().UTC()

	// times shouldn't be exact
	if rtNow.Equal(atNow) {
		t.Errorf("rt.UtcNow() = time.Now.UTC(), %v = %v, should be slightly different", rtNow, atNow)
	}

	rtNowPlusOne := atNow.Add(1 * time.Second)
	rtNowMinusOne := atNow.Add(-1 * time.Second)

	// but should be pretty close
	if atNow.After(rtNowPlusOne) || atNow.Before(rtNowMinusOne) {
		t.Errorf("timedelta between rt.UtcNow() and time.Now.UTC() greater than 2 seconds, %v, %v", rtNow, atNow)
	}
}

func TestFreezeTimeUtcNow(t *testing.T) {
	tm := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	ft := FreezedTime{tm}

	if !tm.Equal(ft.UtcNow()) {
		t.Errorf("ft.UtcNow() != time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC), %v, %v", tm, ft)
	}
}
