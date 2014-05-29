// Simultaneous connection limiter
package connlimit

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/errors"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"net/http"
	"sync"
)

// This limiter tracks concurrent connection per token
// and is capable of rejecting connections if they are failed
type ConnectionLimiter struct {
	mutex            *sync.Mutex
	mapper           MapperFn
	connections      map[string]int
	maxConnections   int
	totalConnections int64
}

func NewClientIpLimiter(maxConnections int) (*ConnectionLimiter, error) {
	return NewConnectionLimiter(MapClientIp, maxConnections)
}

func NewConnectionLimiter(mapper MapperFn, maxConnections int) (*ConnectionLimiter, error) {
	if mapper == nil {
		return nil, fmt.Errorf("Mapper function can not be nil")
	}
	if maxConnections <= 0 {
		return nil, fmt.Errorf("Max connections should be >= 0")
	}
	return &ConnectionLimiter{
		mutex:          &sync.Mutex{},
		mapper:         mapper,
		maxConnections: maxConnections,
		connections:    make(map[string]int),
	}, nil
}

func (cl *ConnectionLimiter) ProcessRequest(r Request) (*http.Response, error) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	token, amount, err := cl.mapper(r)
	if err != nil {
		return nil, err
	}

	connections := cl.connections[token]
	if connections >= cl.maxConnections {
		return netutils.NewTextResponse(
			r.GetHttpRequest(),
			errors.StatusTooManyRequests,
			fmt.Sprintf("Connection limit reached. Max is: %d, yours: %d", cl.maxConnections, connections)), nil
	}

	cl.connections[token] += amount
	cl.totalConnections += int64(amount)
	return nil, nil
}

func (cl *ConnectionLimiter) ProcessResponse(r Request, a Attempt) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	token, amount, err := cl.mapper(r)
	if err != nil {
		return
	}
	cl.connections[token] -= amount
	cl.totalConnections -= int64(amount)

	// Otherwise it would grow forever
	if cl.connections[token] == 0 {
		delete(cl.connections, token)
	}
}

func (cl *ConnectionLimiter) GetConnectionCount() int64 {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()
	return cl.totalConnections
}

func (cl *ConnectionLimiter) GetMaxConnections() int {
	return cl.maxConnections
}

func (cl *ConnectionLimiter) SetMaxConnections(max int) {
	cl.maxConnections = max
}
