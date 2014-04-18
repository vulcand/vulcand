package connwatch

import (
	log "github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/gotools-time"
	. "github.com/mailgun/vulcan/request"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// This limiter tracks concurrent connection per endpoint
// and provides "drain off" capabilities
type ConnectionWatcher struct {
	timeProvider timetools.TimeProvider
	mutex        *sync.RWMutex
	connections  map[string]int
}

func NewConnectionWatcher() *ConnectionWatcher {
	return &ConnectionWatcher{
		timeProvider: &timetools.RealTime{},
		mutex:        &sync.RWMutex{},
		connections:  make(map[string]int),
	}
}

func (cw *ConnectionWatcher) Before(r Request) (*http.Response, error) {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()

	endpoint := getEndpoint(r)
	cw.connections[endpoint] += 1
	log.Infof("endpoint(url=%s) connections: %d", endpoint, cw.connections[endpoint])
	return nil, nil
}

func (cw *ConnectionWatcher) After(r Request) error {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()

	endpoint := getEndpoint(r)
	cw.connections[endpoint] -= 1
	log.Infof("Connections per %s = %d", endpoint, cw.connections[endpoint])
	return nil
}

func (cw *ConnectionWatcher) GetConnectionsCount(endpoint *url.URL) (int, error) {
	cw.mutex.RLock()
	defer cw.mutex.RUnlock()
	return cw.connections[endpoint.Host], nil
}

func (cw *ConnectionWatcher) DrainConnections(timeout time.Duration, endpoints ...*url.URL) (int, error) {
	totalConns := 0
	start := cw.timeProvider.UtcNow()
	for {
		totalConns = 0
		for _, endpoint := range endpoints {
			conns, err := cw.GetConnectionsCount(endpoint)
			if err != nil {
				return totalConns, err
			}
			totalConns += conns
		}
		if totalConns == 0 {
			return 0, nil
		}
		if cw.timeProvider.UtcNow().Sub(start) > timeout {
			return totalConns, nil
		}
		time.Sleep(time.Second)
	}
	return totalConns, nil
}

func getEndpoint(req Request) string {
	return req.GetHttpRequest().URL.Host
}
