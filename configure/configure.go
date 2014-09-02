package configure

import (
	"fmt"

	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"

	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/server"
)

// Configurator watches changes to the dynamic backends and applies those changes to the proxy in real time.
type Configurator struct {
	srv server.Server
}

func NewConfigurator(srv server.Server) (c *Configurator) {
	return &Configurator{
		srv: srv,
	}
}

func (c *Configurator) WatchChanges(changes chan interface{}) error {
	for {
		change := <-changes
		if err := c.processChange(c.srv, change); err != nil {
			log.Errorf("Failed to process change %#v, err: %s", change, err)
		}
	}
	return nil
}

func (c *Configurator) processChange(s server.Server, ch interface{}) error {
	switch change := ch.(type) {
	case *backend.HostAdded:
		return s.UpsertHost(change.Host)
	case *backend.HostDeleted:
		return s.DeleteHost(change.Name)
	case *backend.HostCertUpdated:
		return s.UpdateHostCert(change.Host.Name, change.Host.Cert)
	case *backend.HostListenerAdded:
		return s.AddHostListener(change.Host, change.Listener)
	case *backend.HostListenerDeleted:
		return s.DeleteHostListener(change.Host, change.ListenerId)
	case *backend.LocationAdded:
		return s.UpsertLocation(change.Host, change.Location)
	case *backend.LocationDeleted:
		return s.DeleteLocation(change.Host, change.LocationId)
	case *backend.LocationUpstreamUpdated:
		return s.UpdateLocationUpstream(change.Host, change.Location)
	case *backend.LocationPathUpdated:
		return s.UpdateLocationPath(change.Host, change.Location, change.Path)
	case *backend.LocationOptionsUpdated:
		return s.UpdateLocationOptions(change.Host, change.Location)
	case *backend.LocationMiddlewareAdded:
		return s.UpsertLocationMiddleware(change.Host, change.Location, change.Middleware)
	case *backend.LocationMiddlewareUpdated:
		return s.UpsertLocationMiddleware(change.Host, change.Location, change.Middleware)
	case *backend.LocationMiddlewareDeleted:
		return s.DeleteLocationMiddleware(change.Host, change.Location, change.MiddlewareType, change.MiddlewareId)
	case *backend.UpstreamAdded:
		return nil
	case *backend.UpstreamDeleted:
		return nil
	case *backend.EndpointAdded:
		return s.AddEndpoint(change.Upstream, change.Endpoint, change.AffectedLocations)
	case *backend.EndpointUpdated:
		return s.AddEndpoint(change.Upstream, change.Endpoint, change.AffectedLocations)
	case *backend.EndpointDeleted:
		return s.DeleteEndpoint(change.Upstream, change.EndpointId, change.AffectedLocations)
	}
	return fmt.Errorf("unsupported change: %#v", ch)
}
