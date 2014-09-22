package server

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/metrics"
	"net"
	"net/http"
	"sync"
)

type connTracker struct {
	mtx *sync.Mutex

	c metrics.Client

	new    map[string]int64
	active map[string]int64
	idle   map[string]int64
}

func newConnTracker(c metrics.Client) *connTracker {
	return &connTracker{
		c:      c,
		mtx:    &sync.Mutex{},
		new:    make(map[string]int64),
		active: make(map[string]int64),
		idle:   make(map[string]int64),
	}
}

func (c *connTracker) onStateChange(conn net.Conn, prev http.ConnState, cur http.ConnState) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if cur == http.StateNew || cur == http.StateIdle || cur == http.StateActive {
		c.inc(conn, cur, 1)
	}

	if cur != http.StateNew {
		c.inc(conn, prev, -1)
	}
}

func (c *connTracker) inc(conn net.Conn, state http.ConnState, v int64) {
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
	c.c.Gauge(metric("conns", escape(addr), state.String()), m[addr], 1)
}
