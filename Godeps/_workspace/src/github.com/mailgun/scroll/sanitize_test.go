package scroll

import (
	"fmt"
	"testing"
)

var _ = fmt.Printf // for testing

func TestAllowSetBytes(t *testing.T) {
	tests := []struct {
		inString        string
		inAllow         AllowSet
		outDidReturnErr bool
	}{
		// 0 - no match
		{
			"hello0",
			NewAllowSetBytes(`0123456789`, 100),
			true,
		},
		// 1 - length (input length is one more than max)
		{
			"hello",
			NewAllowSetBytes(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`, 4),
			true,
		},
		// 2 - length (equal)
		{
			"hello",
			NewAllowSetBytes(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`, 5),
			false,
		},
		// 3 - length (input length is one less than max)
		{
			"hello",
			NewAllowSetBytes(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`, 6),
			false,
		},
		// 5 - all good
		{
			"hello, world",
			NewAllowSetBytes(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ, `, 100),
			false,
		},
	}

	for i, tt := range tests {
		if g, w := parseError(tt.inAllow.IsSafe(tt.inString)), tt.outDidReturnErr; g != w {
			t.Errorf("Test(%v), Got IsSafe: %v, Want: %v", i, g, w)
		}
	}
}

func TestAllowSetStrings(t *testing.T) {
	tests := []struct {
		inString        string
		inAllow         AllowSet
		outDidReturnErr bool
	}{
		// 0 - no match
		{
			"foo",
			NewAllowSetStrings([]string{`bar`}),
			true,
		},
		// 1 - empty
		{
			"foo",
			NewAllowSetStrings([]string{``}),
			true,
		},
		// 2 - one less
		{
			"foo",
			NewAllowSetStrings([]string{`fo`}),
			true,
		},
		// 3 - one more
		{
			"foo",
			NewAllowSetStrings([]string{`fooo`}),
			true,
		},
		// 4 - exact match
		{
			"foo",
			NewAllowSetStrings([]string{`foo`}),
			false,
		},
	}

	for i, tt := range tests {
		if g, w := parseError(tt.inAllow.IsSafe(tt.inString)), tt.outDidReturnErr; g != w {
			t.Errorf("Test(%v), Got IsSafe: %v, Want: %v", i, g, w)
		}
	}
}

func parseError(err error) bool {
	if err != nil {
		return true
	}
	return false
}
