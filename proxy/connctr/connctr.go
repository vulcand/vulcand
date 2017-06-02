package connctr

import (
	"net"
	"net/http"
	"sync"

	"github.com/vulcand/vulcand/conntracker"
)

// T represents an active connection counter. It is supposed to be set as a
// state change listener to an http.Server.
type T struct {
	mu     sync.Mutex
	new    map[string]int64
	active map[string]int64
	idle   map[string]int64
}

// New returns a new connection counter instance
func New() *T {
	return &T{
		new:    make(map[string]int64),
		active: make(map[string]int64),
		idle:   make(map[string]int64),
	}
}

// RegisterStateChange implements conntracker.ConnectionTracker.
func (c *T) RegisterStateChange(conn net.Conn, prev http.ConnState, cur http.ConnState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cur == http.StateNew || cur == http.StateIdle || cur == http.StateActive {
		c.inc(conn, cur, 1)
	}

	if cur != http.StateNew {
		c.inc(conn, prev, -1)
	}
}

// Counts implements conntracker.ConnectionTracker.
func (c *T) Counts() conntracker.ConnectionStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	return conntracker.ConnectionStats{
		http.StateNew:    c.copy(c.new),
		http.StateActive: c.copy(c.active),
		http.StateIdle:   c.copy(c.idle),
	}
}

func (c *T) inc(conn net.Conn, state http.ConnState, v int64) {
	addr := conn.LocalAddr().String()
	var m map[string]int64

	switch state {
	case http.StateNew:
		m = c.new
	case http.StateActive:
		m = c.active
	case http.StateIdle:
		m = c.idle
	default:
		return
	}

	m[addr] += v
}

func (c *T) copy(s map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}
