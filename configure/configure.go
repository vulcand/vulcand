package service

import (
	"fmt"
	log "github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/callback"
	"github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcan/route/pathroute"
	. "github.com/mailgun/vulcand/adapter"
	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/endpoint"
	"strings"
)

// Configurator watches changes to the dynamic backends and applies those changes to the proxy in real time.
type Configurator struct {
	proxy *vulcan.Proxy
	a     *Adapter
}

func NewConfigurator(proxy *vulcan.Proxy) (c *Configurator) {
	return &Configurator{
		proxy: proxy,
		a:     NewAdapter(proxy),
	}
}

func (c *Configurator) WatchChanges(changes chan interface{}) error {
	for {
		change := <-changes
		if err := c.processChange(change); err != nil {
			log.Errorf("Failed to process change %s, err: %s", err)
		}
	}
	return nil
}

func (c *Configurator) processChange(ch interface{}) error {
	switch change := ch.(type) {
	case *HostAdded:
		return c.addHost(change.Host)
	case *HostDeleted:
		return c.deleteHost(change.Name)
	case *LocationAdded:
		return c.addLocation(change.Host, change.Location)
	case *LocationDeleted:
		return c.deleteLocation(change.Host, change.LocationId)
	case *LocationUpstreamUpdated:
		return c.syncLocationEndpoints(change.Location)
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
	case *EndpointDeleted:
		return c.deleteEndpoint(change.Upstream, change.EndpointId, change.AffectedLocations)
	}
	return fmt.Errorf("Unsupported change: %#v", ch)
}

func (c *Configurator) addHost(host *Host) error {
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

func (c *Configurator) addLocation(host *Host, loc *Location) error {
	router, err := c.a.GetPathRouter(host.Name)
	if err != nil {
		return err
	}
	// Create a load balancer that handles all the endpoints within the given location
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return err
	}

	before := callback.NewBeforeChain()
	after := callback.NewAfterChain()
	options := httploc.Options{
		Before: before,
		After:  after,
	}

	// Create a location itself
	location, err := httploc.NewLocationWithOptions(loc.Id, rr, options)
	if err != nil {
		return err
	}
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
	location, err := c.a.GetHttpLocation(host.Name, loc.Id)
	if err != nil {
		return err
	}
	limiter, err := NewConnLimiter(cl)
	if err != nil {
		return err
	}
	before := location.GetBefore().(*callback.BeforeChain)
	after := location.GetAfter().(*callback.AfterChain)

	before.Upsert(cl.EtcdKey, limiter)
	after.Upsert(cl.EtcdKey, limiter)
	return nil
}

func (c *Configurator) upsertLocationRateLimit(host *Host, loc *Location, rl *RateLimit) error {
	location, err := c.a.GetHttpLocation(host.Name, loc.Id)
	if err != nil {
		return err
	}
	limiter, err := NewRateLimiter(rl)
	if err != nil {
		return err
	}
	before := location.GetBefore().(*callback.BeforeChain)
	after := location.GetAfter().(*callback.AfterChain)

	before.Upsert(rl.EtcdKey, limiter)
	after.Update(rl.EtcdKey, limiter)
	return nil
}

func (c *Configurator) deleteLocationRateLimit(host *Host, loc *Location, limitId string) error {
	location, err := c.a.GetHttpLocation(host.Name, loc.Id)
	if err != nil {
		return err
	}
	before := location.GetBefore().(*callback.BeforeChain)
	after := location.GetAfter().(*callback.AfterChain)

	if err := before.Remove(limitId); err != nil {
		log.Errorf("Failed to remove limiter: %s")
	}
	return after.Remove(limitId)
}

func (c *Configurator) deleteLocationConnLimit(host *Host, loc *Location, limitId string) error {
	location, err := c.a.GetHttpLocation(host.Name, loc.Id)
	if err != nil {
		return err
	}
	before := location.GetBefore().(*callback.BeforeChain)
	after := location.GetAfter().(*callback.AfterChain)

	if err := before.Remove(limitId); err != nil {
		log.Errorf("Failed to remove limiter: %s")
	}
	return after.Remove(limitId)
}

func (c *Configurator) syncLocationEndpoints(location *Location) error {
	rr, err := c.a.GetHttpLocationLb(location.Hostname, location.Id)
	if err != nil {
		return err
	}

	// First, collect and parse endpoints to add
	endpointsToAdd := map[string]endpoint.Endpoint{}
	for _, e := range location.Upstream.Endpoints {
		ep, err := EndpointFromUrl(e.Id, e.Url)
		if err != nil {
			return fmt.Errorf("Failed to parse endpoint url: %s", e)
		}
		endpointsToAdd[ep.GetId()] = ep
	}

	// Memorize what endpoints exist in load balancer at the moment
	existing := map[string]endpoint.Endpoint{}
	for _, e := range rr.GetEndpoints() {
		existing[e.GetId()] = e
	}

	// First, add endpoints, that should be added and are not in lb
	for eid, e := range endpointsToAdd {
		if _, exists := existing[eid]; !exists {
			if err := rr.AddEndpoint(e); err != nil {
				log.Errorf("Failed to add %s, err: %s", e, err)
			} else {
				log.Infof("Added %s to %s", e, location)
			}
		}
	}

	// Second, remove endpoints that should not be there any more
	for eid, e := range existing {
		if _, exists := endpointsToAdd[eid]; !exists {
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
