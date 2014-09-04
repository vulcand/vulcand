package configure

import (
	"fmt"
	"sync"
	"time"

	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	timetools "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-time"
	"github.com/mailgun/vulcand/connwatch"

	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/server"
)

// Configurator watches changes to the dynamic backends and applies those changes to the proxy in real time.
type Configurator struct {
	lastId       int
	wg           *sync.WaitGroup
	newSrvFn     server.NewServerFn
	srv          server.Server
	backend      backend.Backend
	timeProvider timetools.TimeProvider
	errorC       chan error
	rekickC      chan error
	closeC       chan bool
}

func NewConfigurator(newSrvFn server.NewServerFn, backend backend.Backend, errorC chan error, timeProvider timetools.TimeProvider) (c *Configurator) {
	return &Configurator{
		wg:           &sync.WaitGroup{},
		newSrvFn:     newSrvFn,
		backend:      backend,
		timeProvider: timeProvider,
		errorC:       errorC,
		rekickC:      make(chan error),
	}
}

func (c *Configurator) GetConnWatcher() *connwatch.ConnectionWatcher {
	return c.srv.GetConnWatcher()
}

func (c *Configurator) GetStats(hostname, locationId string, e *backend.Endpoint) *backend.EndpointStats {
	return c.srv.GetStats(hostname, locationId, e)
}

func (c *Configurator) init() error {
	srv, err := c.newSrvFn(c.lastId)
	if err != nil {
		return err
	}
	c.lastId += 1

	if err := c.initialSetup(srv); err != nil {
		return err
	}

	log.Infof("init() initial setup for %s is done", srv)

	oldSrv := c.srv
	if oldSrv != nil {
		log.Infof("init() hijacking listeners from %s for %s", oldSrv, srv)
		if err := srv.HijackListeners(oldSrv); err != nil {
			return err
		}
	}

	if err := srv.Start(); err != nil {
		return err
	}

	if oldSrv != nil {
		go func() {
			c.wg.Add(1)
			oldSrv.Stop(true)
			c.wg.Done()
		}()
	}

	// Watch and configure this instance of server
	c.srv = srv
	changesC := make(chan interface{})

	// This goroutine will listen for the changes from backend
	go func() {
		if err := c.backend.WatchChanges(changesC); err != nil {
			log.Infof("Backend watcher got error: '%s', launching reconnect procedure", err)
			close(changesC)
			c.rekickC <- err
		} else {
			// Graceful shutdown without restart
			log.Infof("Backend watcher got nil error, gracefully shutdown")
			close(c.rekickC)
		}
	}()

	// Listen for changes from the backend to configure the newly initated
	go c.watchChanges(srv, changesC)

	return nil
}

func (c *Configurator) supervise() {
	for {
		err := <-c.rekickC
		// This means graceful shutdown, do nothing and return
		if err == nil {
			log.Infof("watchErrors - graceful shutdown")
			return
		}
		for {
			c.timeProvider.Sleep(retryPeriod)
			log.Infof("supervise() restarting %s on error: %s", c.srv, err)
			// We failed to initialize server, this error can not be recovered, so send an error and exit
			if err := c.init(); err != nil {
				log.Infof("Failed to initialize %s, will retry", err)
			} else {
				break
			}
		}
	}
}

func (c *Configurator) Start() error {
	go c.supervise()
	return c.init()
}

func (c *Configurator) Stop(wait bool) {
	// Wait for any outstanding operations to complete
	c.wg.Wait()
	// Wait for current running server to complete
	c.srv.Stop(wait)
}

func (c *Configurator) watchChanges(srv server.Server, changes chan interface{}) {
	for {
		change := <-changes
		if change == nil {
			log.Infof("Stop watching changes for %s", srv)
			return
		}
		if err := c.processChange(srv, change); err != nil {
			log.Errorf("Failed to process change %#v, err: %s", change, err)
		}
	}
}

// Reads the configuration of the vulcand and generates a sequence of events
// just like as someone was creating locations and hosts in sequence.
func (c *Configurator) initialSetup(srv server.Server) error {
	hosts, err := c.backend.GetHosts()
	if err != nil {
		return err
	}

	if len(hosts) == 0 {
		log.Warningf("No hosts found")
	}

	for _, h := range hosts {
		if err := srv.UpsertHost(h); err != nil {
			return err
		}
		for _, l := range h.Locations {
			if err := srv.UpsertLocation(h, l); err != nil {
				return err
			}
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

const retryPeriod = 5 * time.Second
