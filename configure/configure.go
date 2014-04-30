package service

import (
	"fmt"
	log "github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcan/route/pathroute"
	. "github.com/mailgun/vulcand/adapter"
	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/connwatch"
	. "github.com/mailgun/vulcand/endpoint"
	"strings"
)

const ConnWatch = "_vulcanConnWatch"

// Configurator watches changes to the dynamic backends and applies those changes to the proxy in real time.
type Configurator struct {
	connWatcher *ConnectionWatcher
	proxy       *vulcan.Proxy
	a           *Adapter
}

func NewConfigurator(proxy *vulcan.Proxy) (c *Configurator) {
	return &Configurator{
		proxy:       proxy,
		a:           NewAdapter(proxy),
		connWatcher: NewConnectionWatcher(),
	}
}

func (c *Configurator) GetConnWatcher() *ConnectionWatcher {
	return c.connWatcher
}

func (c *Configurator) WatchChanges(changes chan interface{}) error {
	for {
		change := <-changes
		if err := c.processChange(change); err != nil {
			log.Errorf("Failed to process change %#v, err: %s", change, err)
		}
	}
	return nil
}

func (c *Configurator) processChange(ch interface{}) error {
	switch change := ch.(type) {
	case *HostAdded:
		return c.upsertHost(change.Host)
	case *HostDeleted:
		return c.deleteHost(change.Name)
	case *LocationAdded:
		return c.upsertLocation(change.Host, change.Location)
	case *LocationDeleted:
		return c.deleteLocation(change.Host, change.LocationId)
	case *LocationUpstreamUpdated:
		return c.updateLocationUpstream(change.Host, change.Location)
	case *LocationPathUpdated:
		return c.updateLocationPath(change.Host, change.Location, change.Path)
	case *LocationRateLimitAdded:
		return c.upsertLocationRateLimit(change.Host, change.Location, change.RateLimit)
	case *LocationRateLimitUpdated:
		return c.upsertLocationRateLimit(change.Host, change.Location, change.RateLimit)
	case *LocationRateLimitDeleted:
		return c.deleteLocationRateLimit(change.Host, change.Location, change.RateLimitId)
	case *LocationConnLimitAdded:
		return c.upsertLocationConnLimit(change.Host, change.Location, change.ConnLimit)
	case *LocationConnLimitUpdated:
		return c.upsertLocationConnLimit(change.Host, change.Location, change.ConnLimit)
	case *LocationConnLimitDeleted:
		return c.deleteLocationConnLimit(change.Host, change.Location, change.ConnLimitId)
	case *UpstreamAdded:
		return nil
	case *UpstreamDeleted:
		return nil
	case *EndpointAdded:
		return c.addEndpoint(change.Upstream, change.Endpoint, change.AffectedLocations)
	case *EndpointUpdated:
		return c.addEndpoint(change.Upstream, change.Endpoint, change.AffectedLocations)
	case *EndpointDeleted:
		return c.deleteEndpoint(change.Upstream, change.EndpointId, change.AffectedLocations)
	}
	return fmt.Errorf("Unsupported change: %#v", ch)
}

func (c *Configurator) upsertHost(host *Host) error {
	router := pathroute.NewPathRouter()
	c.a.GetHostRouter().SetRouter(host.Name, router)
	log.Infof("Added %s", host)
	return nil
}

func (c *Configurator) deleteHost(hostname string) error {
	log.Infof("Removed host %s", hostname)
	c.a.GetHostRouter().RemoveRouter(hostname)
	return nil
}

func (c *Configurator) upsertLocation(host *Host, loc *Location) error {
	if err := c.upsertHost(host); err != nil {
		return err
	}

	// If location already exists, do nothing
	if loc, err := c.a.FindHttpLocation(host.Name, loc.Id); err != nil || loc != nil {
		return nil
	}

	router, err := c.a.GetPathRouter(host.Name)
	if err != nil {
		return err
	}
	// Create a load balancer that handles all the endpoints within the given location
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return err
	}

	// Create a location itself
	location, err := httploc.NewLocation(loc.Id, rr)
	if err != nil {
		return err
	}

	// Always register a global connection watcher
	location.GetObserverChain().Upsert(ConnWatch, c.connWatcher)

	// Add the location to the router
	if err := router.AddLocation(loc.Path, location); err != nil {
		return err
	}
	// Add rate and connection limits
	for _, rl := range loc.RateLimits {
		if err := c.upsertLocationRateLimit(host, loc, rl); err != nil {
			log.Errorf("Failed to add rate limit: %s", err)
		}
	}
	for _, cl := range loc.ConnLimits {
		if err := c.upsertLocationConnLimit(host, loc, cl); err != nil {
			log.Errorf("Failed to add connection limit: %s", err)
		}
	}
	// Once the location added, configure all endpoints
	return c.syncLocationEndpoints(loc)
}

func (c *Configurator) deleteLocation(host *Host, locationId string) error {
	router, err := c.a.GetPathRouter(host.Name)
	if err != nil {
		return err
	}
	location := router.GetLocationById(locationId)
	if location == nil {
		return fmt.Errorf("Location(id=%s) not found", locationId)
	}
	err = router.RemoveLocation(location)
	if err == nil {
		log.Infof("Deleted location(id=%s)", locationId)
	}
	return err
}

func (c *Configurator) upsertLocationConnLimit(host *Host, loc *Location, cl *ConnLimit) error {
	if err := c.upsertLocation(host, loc); err != nil {
		return err
	}

	location, err := c.a.GetHttpLocation(host.Name, loc.Id)
	if err != nil {
		return err
	}
	limiter, err := NewConnLimiter(cl)
	if err != nil {
		return err
	}

	location.GetMiddlewareChain().Upsert(cl.EtcdKey, limiter)
	return nil
}

func (c *Configurator) upsertLocationRateLimit(host *Host, loc *Location, rl *RateLimit) error {

	if err := c.upsertLocation(host, loc); err != nil {
		return err
	}

	location, err := c.a.GetHttpLocation(host.Name, loc.Id)
	if err != nil {
		return err
	}
	limiter, err := NewRateLimiter(rl)
	if err != nil {
		return err
	}

	location.GetMiddlewareChain().Upsert(rl.EtcdKey, limiter)
	return nil
}

func (c *Configurator) deleteLocationRateLimit(host *Host, loc *Location, limitId string) error {
	location, err := c.a.GetHttpLocation(host.Name, loc.Id)
	if err != nil {
		return err
	}
	return location.GetMiddlewareChain().Remove(limitId)
}

func (c *Configurator) deleteLocationConnLimit(host *Host, loc *Location, limitId string) error {
	location, err := c.a.GetHttpLocation(host.Name, loc.Id)
	if err != nil {
		return err
	}
	return location.GetMiddlewareChain().Remove(limitId)
}

func (c *Configurator) updateLocationPath(host *Host, location *Location, path string) error {
	if err := c.deleteLocation(host, location.Id); err != nil {
		return err
	}
	return c.upsertLocation(host, location)
}

func (c *Configurator) updateLocationUpstream(host *Host, location *Location) error {
	if err := c.upsertLocation(host, location); err != nil {
		return err
	}
	return c.syncLocationEndpoints(location)
}

func (c *Configurator) syncLocationEndpoints(location *Location) error {

	rr, err := c.a.GetHttpLocationLb(location.Hostname, location.Id)
	if err != nil {
		return err
	}

	// First, collect and parse endpoints to add
	newEndpoints := map[string]endpoint.Endpoint{}
	for _, e := range location.Upstream.Endpoints {
		ep, err := EndpointFromUrl(e.Id, e.Url)
		if err != nil {
			return fmt.Errorf("Failed to parse endpoint url: %s", e)
		}
		newEndpoints[ep.GetId()] = ep
	}

	// Memorize what endpoints exist in load balancer at the moment
	existingEndpoints := map[string]endpoint.Endpoint{}
	for _, e := range rr.GetEndpoints() {
		existingEndpoints[e.GetId()] = e
	}

	// First, add endpoints, that should be added and are not in lb
	for eid, e := range newEndpoints {
		if _, exists := existingEndpoints[eid]; !exists {
			if err := rr.AddEndpoint(e); err != nil {
				log.Errorf("Failed to add %s, err: %s", e, err)
			} else {
				log.Infof("Added %s to %s", e, location)
			}
		}
	}

	// Second, remove endpoints that should not be there any more
	for eid, e := range existingEndpoints {
		if _, exists := newEndpoints[eid]; !exists {
			if err := rr.RemoveEndpoint(e); err != nil {
				log.Errorf("Failed to remove %s, err: %s", e, err)
			} else {
				log.Infof("Removed %s from %s", e, location)
			}
		}
	}
	return nil
}

func (c *Configurator) addEndpoint(upstream *Upstream, e *Endpoint, affectedLocations []*Location) error {
	endpoint, err := EndpointFromUrl(e.Id, e.Url)
	if err != nil {
		return fmt.Errorf("Failed to parse endpoint url: %s", endpoint)
	}
	for _, l := range affectedLocations {
		if err := c.syncLocationEndpoints(l); err != nil {
			log.Errorf("Failed to sync %s endpoints err: %s", l, err)
		}
	}
	return nil
}

func (c *Configurator) deleteEndpoint(upstream *Upstream, endpointId string, affectedLocations []*Location) error {
	for _, l := range affectedLocations {
		if err := c.syncLocationEndpoints(l); err != nil {
			log.Errorf("Failed to sync %s endpoints err: %s", l, err)
		}
	}
	return nil
}

func join(vals ...string) string {
	return strings.Join(vals, ",")
}
