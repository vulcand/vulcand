package server

import (
	"fmt"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"

	"github.com/mailgun/vulcand/backend"
)

type location struct {
	m    *MuxServer
	loc  backend.Location
	hloc *httploc.HttpLocation
	up   *upstream
}

func (l *location) getLB() *roundrobin.RoundRobin {
	return l.hloc.GetLoadBalancer().(*roundrobin.RoundRobin)
}

func (l *location) updateUpstream(up *upstream) error {
	oldup := l.up
	l.up = up

	// Switching upstreams, set the new transport and perform switch
	if up.up.Id != oldup.up.Id {
		oldup.deleteLocation(l.loc.GetUniqueId())
		up.addLocation(l.loc.GetUniqueId(), l)
		l.hloc.SetTransport(up.t)
	}

	return l.syncEndpoints()
}

func newLocation(m *MuxServer, loc *backend.Location, up *upstream) (*location, error) {
	router := m.getRouter(loc.Hostname)
	if router == nil {
		return nil, fmt.Errorf("router not found for %s", loc.Hostname)
	}

	// Create a load balancer that handles all the endpoints within the given location
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return nil, err
	}

	// Create a http location
	options, err := loc.GetOptions()
	if err != nil {
		return nil, err
	}

	// Use the transport from the upstream
	options.Transport = up.t
	hloc, err := httploc.NewLocationWithOptions(loc.Id, rr, *options)
	if err != nil {
		return nil, err
	}

	// Register metric emitters and performance monitors
	hloc.GetObserverChain().Upsert(Metrics, NewReporter(m.options.MetricsClient, loc.Id))
	hloc.GetObserverChain().Upsert(PerfMon, m.perfMon)

	// Add the location to the router
	if err := router.AddLocation(loc.Path, hloc); err != nil {
		return nil, err
	}

	l := &location{
		hloc: hloc,
		loc:  *loc,
		m:    m,
		up:   up,
	}

	// Add middlewares
	for _, ml := range loc.Middlewares {
		if err := l.upsertMiddleware(ml); err != nil {
			log.Errorf("failed to add middleware: %s", err)
		}
	}

	// Once the location added, configure all endpoints
	if err := l.syncEndpoints(); err != nil {
		return nil, err
	}

	// Link the location and the upstream
	up.addLocation(l.loc.GetUniqueId(), l)

	return l, nil
}

func (l *location) syncEndpoints() error {
	rr := l.getLB()
	if rr == nil {
		return fmt.Errorf("%v lb not found", l.loc)
	}

	// First, collect and parse endpoints to add
	newEndpoints := map[string]*muxEndpoint{}
	for _, e := range l.up.up.Endpoints {
		ep, err := newEndpoint(&l.loc, e, l.m.perfMon)
		if err != nil {
			return fmt.Errorf("failed to create load balancer endpoint from %v", e)
		}
		newEndpoints[e.Url] = ep
	}

	// Memorize what endpoints exist in load balancer at the moment
	existingEndpoints := map[string]*muxEndpoint{}
	for _, e := range rr.GetEndpoints() {
		existingEndpoints[e.GetUrl().String()] = e.GetOriginalEndpoint().(*muxEndpoint)
	}

	// First, add endpoints, that should be added and are not in lb
	for _, e := range newEndpoints {
		if _, exists := existingEndpoints[e.GetUrl().String()]; !exists {
			if err := rr.AddEndpoint(e); err != nil {
				log.Errorf("%v failed to add %v, err: %s", l.m, e, err)
			} else {
				log.Infof("%v add %v to %v", l.m, e, &l.loc)
			}
		}
	}

	// Second, remove endpoints that should not be there any more
	for _, e := range existingEndpoints {
		if _, exists := newEndpoints[e.GetUrl().String()]; !exists {
			l.m.perfMon.deleteEndpoint(e.endpoint.GetUniqueId())
			if err := rr.RemoveEndpoint(e); err != nil {
				log.Errorf("%v failed to remove %v, err: %v", l.m, e, err)
			} else {
				log.Infof("%v removed %v from %v", l.m, e, &l.loc)
			}
		}
	}
	return nil
}

func (l *location) upsertMiddleware(mi *backend.MiddlewareInstance) error {
	instance, err := mi.Middleware.NewMiddleware()
	if err != nil {
		return err
	}
	l.hloc.GetMiddlewareChain().Upsert(fmt.Sprintf("%s.%s", mi.Type, mi.Id), mi.Priority, instance)
	return nil
}

func (l *location) deleteMiddleware(mType, mId string) error {
	return l.hloc.GetMiddlewareChain().Remove(fmt.Sprintf("%s.%s", mType, mId))
}

func (l *location) updateOptions(loc *backend.Location) error {
	l.loc = *loc
	options, err := loc.GetOptions()
	if err != nil {
		return err
	}
	options.Transport = l.up.t
	return l.hloc.SetOptions(*options)
}

func (l *location) remove() error {
	router := l.m.getRouter(l.loc.Hostname)
	if router == nil {
		return fmt.Errorf("router for %s not found", l.loc.Hostname)
	}
	l.m.perfMon.deleteLocation(l.loc.GetUniqueId())
	l.up.deleteLocation(l.loc.GetUniqueId())
	return router.RemoveLocationByExpression(l.loc.Path)
}
