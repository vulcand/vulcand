package configure

import (
	"fmt"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/endpoint"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route/exproute"

	"github.com/mailgun/vulcand/adapter"
	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/connwatch"
	. "github.com/mailgun/vulcand/endpoint"
	"strings"
	"time"
)

const ConnWatch = "_vulcanConnWatch"

type Options struct {
	DialTimeout time.Duration
	ReadTimeout time.Duration
}

// Configurator watches changes to the dynamic backends and applies those changes to the proxy in real time.
type Configurator struct {
	options     Options
	connWatcher *connwatch.ConnectionWatcher
	proxy       *vulcan.Proxy
	a           *adapter.Adapter
}

func NewConfigurator(proxy *vulcan.Proxy) (c *Configurator) {
	return NewConfiguratorWithOptions(proxy, Options{})
}

func NewConfiguratorWithOptions(proxy *vulcan.Proxy, options Options) (c *Configurator) {
	return &Configurator{
		proxy:       proxy,
		a:           adapter.NewAdapter(proxy),
		connWatcher: connwatch.NewConnectionWatcher(),
		options:     options,
	}
}

func (c *Configurator) GetConnWatcher() *connwatch.ConnectionWatcher {
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
	case *backend.HostAdded:
		return c.upsertHost(change.Host)
	case *backend.HostDeleted:
		return c.deleteHost(change.Name)
	case *backend.LocationAdded:
		return c.upsertLocation(change.Host, change.Location)
	case *backend.LocationDeleted:
		return c.deleteLocation(change.Host, change.LocationId)
	case *backend.LocationUpstreamUpdated:
		return c.updateLocationUpstream(change.Host, change.Location)
	case *backend.LocationPathUpdated:
		return c.updateLocationPath(change.Host, change.Location, change.Path)
	case *backend.LocationOptionsUpdated:
		return c.updateLocationOptions(change.Host, change.Location)
	case *backend.LocationMiddlewareAdded:
		return c.upsertLocationMiddleware(change.Host, change.Location, change.Middleware)
	case *backend.LocationMiddlewareUpdated:
		return c.upsertLocationMiddleware(change.Host, change.Location, change.Middleware)
	case *backend.LocationMiddlewareDeleted:
		return c.deleteLocationMiddleware(change.Host, change.Location, change.MiddlewareType, change.MiddlewareId)
	case *backend.UpstreamAdded:
		return nil
	case *backend.UpstreamDeleted:
		return nil
	case *backend.EndpointAdded:
		return c.addEndpoint(change.Upstream, change.Endpoint, change.AffectedLocations)
	case *backend.EndpointUpdated:
		return c.addEndpoint(change.Upstream, change.Endpoint, change.AffectedLocations)
	case *backend.EndpointDeleted:
		return c.deleteEndpoint(change.Upstream, change.EndpointId, change.AffectedLocations)
	}
	return fmt.Errorf("unsupported change: %#v", ch)
}

func (c *Configurator) upsertHost(host *backend.Host) error {
	if c.a.GetHostRouter().GetRouter(host.Name) != nil {
		return nil
	}
	router := exproute.NewExpRouter()
	c.a.GetHostRouter().SetRouter(host.Name, router)
	log.Infof("Added %s", host)
	return nil
}

func (c *Configurator) deleteHost(hostname string) error {
	log.Infof("Removed host %s", hostname)
	c.a.GetHostRouter().RemoveRouter(hostname)
	return nil
}

func (c *Configurator) getLocationOptions(loc *backend.Location) (*httploc.Options, error) {
	o, err := loc.GetOptions()
	if err != nil {
		return nil, err
	}

	// Apply global defaults if options are not set
	if o.Timeouts.Dial == 0 {
		o.Timeouts.Dial = c.options.DialTimeout
	}
	if o.Timeouts.Read == 0 {
		o.Timeouts.Read = c.options.ReadTimeout
	}
	return o, nil
}

func (c *Configurator) upsertLocation(host *backend.Host, loc *backend.Location) error {
	if err := c.upsertHost(host); err != nil {
		return err
	}

	// If location already exists, do nothing
	if loc := c.a.GetHttpLocation(host.Name, loc.Id); loc != nil {
		return nil
	}

	router := c.a.GetExpRouter(host.Name)
	if router == nil {
		return fmt.Errorf("router not found for %s", host)
	}
	// Create a load balancer that handles all the endpoints within the given location
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return err
	}

	// Create a location itself
	options, err := c.getLocationOptions(loc)
	if err != nil {
		return err
	}
	location, err := httploc.NewLocationWithOptions(loc.Id, rr, *options)
	if err != nil {
		return err
	}

	// Always register a global connection watcher
	location.GetObserverChain().Upsert(ConnWatch, c.connWatcher)

	// Add the location to the router
	if err := router.AddLocation(convertPath(loc.Path), location); err != nil {
		return err
	}

	// Add middlewares
	for _, ml := range loc.Middlewares {
		if err := c.upsertLocationMiddleware(host, loc, ml); err != nil {
			log.Errorf("failed to add middleware: %s", err)
		}
	}
	// Once the location added, configure all endpoints
	return c.syncLocationEndpoints(loc)
}

func (c *Configurator) deleteLocation(host *backend.Host, locationId string) error {

	router := c.a.GetExpRouter(host.Name)
	if router == nil {
		return fmt.Errorf("Router for %s not found", host)
	}

	location := router.GetLocationById(locationId)
	if location == nil {
		return fmt.Errorf("Location(id=%s) not found", locationId)
	}
	return router.RemoveLocationById(location.GetId())
}

func (c *Configurator) updateLocationOptions(host *backend.Host, loc *backend.Location) error {
	log.Infof("Updating location options %s, options: %#v", loc, loc.Options)

	if err := c.upsertLocation(host, loc); err != nil {
		return err
	}
	location := c.a.GetHttpLocation(host.Name, loc.Id)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
	}
	options, err := c.getLocationOptions(loc)
	if err != nil {
		return err
	}
	return location.SetOptions(*options)
}

func (c *Configurator) upsertLocationMiddleware(host *backend.Host, loc *backend.Location, m *backend.MiddlewareInstance) error {
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

func (c *Configurator) deleteLocationMiddleware(host *backend.Host, loc *backend.Location, mType, mId string) error {
	location := c.a.GetHttpLocation(host.Name, loc.Id)
	if location == nil {
		return fmt.Errorf("%s not found", loc)
	}
	return location.GetMiddlewareChain().Remove(fmt.Sprintf("%s.%s", mType, mId))
}

func (c *Configurator) updateLocationPath(host *backend.Host, location *backend.Location, path string) error {
	// If location already exists, delete it and re-create from scratch
	if loc := c.a.GetHttpLocation(host.Name, location.Id); loc != nil {
		if err := c.deleteLocation(host, location.Id); err != nil {
			return err
		}
	}
	return c.upsertLocation(host, location)
}

func (c *Configurator) updateLocationUpstream(host *backend.Host, location *backend.Location) error {
	if err := c.upsertLocation(host, location); err != nil {
		return err
	}
	return c.syncLocationEndpoints(location)
}

func (c *Configurator) syncLocationEndpoints(location *backend.Location) error {

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

func (c *Configurator) addEndpoint(upstream *backend.Upstream, e *backend.Endpoint, affectedLocations []*backend.Location) error {
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

func (c *Configurator) deleteEndpoint(upstream *backend.Upstream, endpointId string, affectedLocations []*backend.Location) error {
	for _, l := range affectedLocations {
		if err := c.syncLocationEndpoints(l); err != nil {
			log.Errorf("Failed to sync %s endpoints err: %s", l, err)
		}
	}
	return nil
}

// convertPath changes strings to structured format /hello -> RegexpRoute("/hello") and leaves structured strings unchanged.
func convertPath(in string) string {
	if !strings.Contains(in, exproute.TrieRouteFn) && !strings.Contains(in, exproute.RegexpRouteFn) {
		return fmt.Sprintf(`%s(%#v)`, exproute.RegexpRouteFn, in)
	}
	return in
}
