// Route the request by hostname
package hostroute

import (
	"fmt"
	. "github.com/mailgun/vulcan/location"
	. "github.com/mailgun/vulcan/request"
	. "github.com/mailgun/vulcan/route"
	"strings"
	"sync"
)

// This router composer helps to match request by host header and uses inner
// routes to do further matching
type HostRouter struct {
	routers map[string]Router
	mutex   *sync.Mutex
}

func NewHostRouter() *HostRouter {
	return &HostRouter{
		mutex:   &sync.Mutex{},
		routers: make(map[string]Router),
	}
}

func (h *HostRouter) Route(req Request) (Location, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	hostname := strings.Split(strings.ToLower(req.GetHttpRequest().Host), ":")[0]
	matcher, exists := h.routers[hostname]
	if !exists {
		return nil, nil
	}
	return matcher.Route(req)
}

func (h *HostRouter) SetRouter(hostname string, router Router) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if router == nil {
		return fmt.Errorf("Router can not be nil")
	}

	h.routers[hostname] = router
	return nil
}

func (h *HostRouter) GetRouter(hostname string) Router {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	router := h.routers[hostname]
	return router
}

func (h *HostRouter) RemoveRouter(hostname string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	delete(h.routers, hostname)
}
