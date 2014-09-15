package statsd

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
)

type Client struct {
	// underlying connection
	c net.PacketConn
	// resolved udp address
	ra *net.UDPAddr
	// prefix for statsd name
	prefix string
}

// Close closes the connection and cleans up.
func (s *Client) Close() error {
	err := s.c.Close()
	return err
}

// Increments a statsd count type.
// stat is a string name for the metric.
// value is the integer value
// rate is the sample rate (0.0 to 1.0)
func (s *Client) Inc(stat string, value int64, rate float32) error {
	dap := fmt.Sprintf("%d|c", value)
	return s.submit(stat, dap, rate)
}

// Decrements a statsd count type.
// stat is a string name for the metric.
// value is the integer value.
// rate is the sample rate (0.0 to 1.0).
func (s *Client) Dec(stat string, value int64, rate float32) error {
	return s.Inc(stat, -value, rate)
}

// Submits/Updates a statsd gauge type.
// stat is a string name for the metric.
// value is the integer value.
// rate is the sample rate (0.0 to 1.0).
func (s *Client) Gauge(stat string, value int64, rate float32) error {
	dap := fmt.Sprintf("%d|g", value)
	return s.submit(stat, dap, rate)
}

// Submits a delta to a statsd gauge.
// stat is the string name for the metric.
// value is the (positive or negative) change.
// rate is the sample rate (0.0 to 1.0).
func (s *Client) GaugeDelta(stat string, value int64, rate float32) error {
	dap := fmt.Sprintf("%+d|g", value)
	return s.submit(stat, dap, rate)
}

// Submits a statsd timing type.
// stat is a string name for the metric.
// value is the integer value.
// rate is the sample rate (0.0 to 1.0).
func (s *Client) Timing(stat string, delta int64, rate float32) error {
	dap := fmt.Sprintf("%d|ms", delta)
	return s.submit(stat, dap, rate)
}

// Submits a stats set type, where value is the unique string
// rate is the sample rate (0.0 to 1.0).
func (s *Client) UniqueString(stat string, value string, rate float32) error {
	dap := fmt.Sprintf("%s|s", value)
	return s.submit(stat, dap, rate)
}

// Submits a stats set type
// rate is the sample rate (0.0 to 1.0).
func (s *Client) UniqueInt64(stat string, value int64, rate float32) error {
	dap := fmt.Sprintf("%d|s", value)
	return s.submit(stat, dap, rate)
}

// Sets/Updates the statsd client prefix
func (s *Client) SetPrefix(prefix string) {
	s.prefix = prefix
}

// submit formats the statsd event data, handles sampling, and prepares it,
// and sends it to the server.
func (s *Client) submit(stat string, value string, rate float32) error {
	if rate < 1 {
		if rand.Float32() < rate {
			value = fmt.Sprintf("%s|@%f", value, rate)
		} else {
			return nil
		}
	}

	if s.prefix != "" {
		stat = fmt.Sprintf("%s.%s", s.prefix, stat)
	}

	data := fmt.Sprintf("%s:%s", stat, value)

	_, err := s.send([]byte(data))
	if err != nil {
		return err
	}
	return nil
}

// sends the data to the server endpoint
func (s *Client) send(data []byte) (int, error) {
	// no need for locking here, as the underlying fdNet
	// already serialized writes
	n, err := s.c.(*net.UDPConn).WriteToUDP([]byte(data), s.ra)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return n, errors.New("Wrote no bytes")
	}
	return n, nil
}

// Returns a pointer to a new Client, and an error.
// addr is a string of the format "hostname:port", and must be parsable by
// net.ResolveUDPAddr.
// prefix is the statsd client prefix. Can be "" if no prefix is desired.
func New(addr, prefix string) (*Client, error) {
	c, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}

	ra, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	client := &Client{
		c:      c,
		ra:     ra,
		prefix: prefix}

	return client, nil
}

// Compat
var Dial = New
