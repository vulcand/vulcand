package configure

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
	case *LocationMiddlewareAdded:
		return c.upsertLocationMiddleware(change.Host, change.Location, change.Middleware)
	case *LocationMiddlewareUpdated:
		return c.upsertLocationMiddleware(change.Host, change.Location, change.Middleware)
	case *LocationMiddlewareDeleted:
		return c.deleteLocationMiddleware(change.Host, change.Location, change.MiddlewareType, change.MiddlewareId)
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
	if c.a.GetHostRouter().GetRouter(host.Name) != nil {
		return nil
	}
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
	if loc := c.a.GetHttpLocation(host.Name, loc.Id); loc != nil {
		return nil
	}

	router := c.a.GetPathRouter(host.Name)
	if router == nil {
		return fmt.Errorf("Router not found for %s", host)
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

	// Add middlewares
	for _, ml := range loc.Middlewares {
		if err := c.upsertLocationMiddleware(host, loc, ml); err != nil {
			log.Errorf("Failed to add middleware: %s", err)
		}
	}
	// Once the location added, configure all endpoints
	return c.syncLocationEndpoints(loc)
}

func (c *Configurator) deleteLocation(host *Host, locationId string) error {

	router := c.a.GetPathRouter(host.Name)
	if router == nil {
		return fmt.Errorf("Router for %s not found", host)
	}

	location := router.GetLocationById(locationId)
	if location == nil {
		return fmt.Errorf("Location(id=%s) not found", locationId)
	}
	return router.RemoveLocation(location)
}

func (c *Configurator) upsertLocationMiddleware(host *Host, loc *Location, m *MiddlewareInstance) error {
	if err := c.upsertLocation(host, loc); err != nil {
		return err
	}
	location := c.a.GetHttpLocation(host.Name, loc.Id)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
	}
	instance, err := m.Middleware.NewMiddleware()
	if err != nil {
		return err
	}
	location.GetMiddlewareChain().Upsert(fmt.Sprintf("%s.%s", m.Type, m.Id), m.Priority, instance)
	return nil
}

func (c *Configurator) deleteLocationMiddleware(host *Host, loc *Location, mType, mId string) error {
	location := c.a.GetHttpLocation(host.Name, loc.Id)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
	}
	return location.GetMiddlewareChain().Remove(fmt.Sprintf("%s.%s", mType, mId))
}

func (c *Configurator) deleteLocationConnLimit(host *Host, loc *Location, limitId string) error {
	location := c.a.GetHttpLocation(host.Name, loc.Id)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
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

	rr := c.a.GetHttpLocationLb(location.Hostname, location.Id)
	if rr == nil {
		return fmt.Errorf("%s lb not found", location)
	}

	// First, collect and parse endpoints to add
	newEndpoints := map[string]endpoint.Endpoint{}
	for _, e := range location.Upstream.Endpoints {
		ep, err := EndpointFromUrl(e.GetUniqueId(), e.Url)
		if err != nil {
			return fmt.Errorf("Failed to parse endpoint url: %s", e)
		}
		newEndpoints[e.Url] = ep
	}

	// Memorize what endpoints exist in load balancer at the moment
	existingEndpoints := map[string]endpoint.Endpoint{}
	for _, e := range rr.GetEndpoints() {
		existingEndpoints[e.GetUrl().String()] = e
	}

	// First, add endpoints, that should be added and are not in lb
	for _, e := range newEndpoints {
		if _, exists := existingEndpoints[e.GetUrl().String()]; !exists {
			if err := rr.AddEndpoint(e); err != nil {
				log.Errorf("Failed to add %s, err: %s", e, err)
			} else {
				log.Infof("Added %s to %s", e, location)
			}
		}
	}

	// Second, remove endpoints that should not be there any more
	for _, e := range existingEndpoints {
		if _, exists := newEndpoints[e.GetUrl().String()]; !exists {
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
	endpoint, err := EndpointFromUrl(e.GetUniqueId(), e.Url)
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
