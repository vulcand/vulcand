package server

import (
	"fmt"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"

	"github.com/mailgun/vulcand/engine"
)

type frontend struct {
	m    *MuxServer
	loc  engine.Frontend
	hloc *httploc.HttpLocation
	b    *backend
}

func (l *frontend) getLB() *roundrobin.RoundRobin {
	return l.hloc.GetLoadBalancer().(*roundrobin.RoundRobin)
}

func (l *frontend) updateUpstream(up *upstream) error {
	oldup := l.up
	l.up = up

	// Switching upstreams, set the new transport and perform switch
	if up.up.Id != oldup.up.Id {
		oldup.deleteFrontend(l.loc.GetUniqueId())
		up.addFrontend(l.loc.GetUniqueId(), l)
		l.hloc.SetTransport(up.t)
	}

	return l.syncEndpoints()
}

func newFrontend(m *MuxServer, loc *backend.Frontend, up *upstream) (*frontend, error) {
	router := m.getRouter(loc.Hostname)
	if router == nil {
		return nil, fmt.Errorf("router not found for %s", loc.Hostname)
	}

	// Create a load balancer that handles all the endpoints within the given frontend
	rr, err := roundrobin.NewRoundRobin()
	if err != nil {
		return nil, err
	}

	// Create a http frontend
	options, err := loc.GetOptions()
	if err != nil {
		return nil, err
	}

	// Use the transport from the upstream
	options.Transport = up.t
	hloc, err := httploc.NewFrontendWithOptions(loc.Id, rr, *options)
	if err != nil {
		return nil, err
	}

	// Register metric emitters and performance monitors
	hloc.GetObserverChain().Upsert(Metrics, NewReporter(m.options.MetricsClient, loc.Id))
	hloc.GetObserverChain().Upsert(PerfMon, m.perfMon)

	// Add the frontend to the router
	if err := router.AddFrontend(loc.Path, hloc); err != nil {
		return nil, err
	}

	l := &frontend{
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

	// Once the frontend added, configure all endpoints
	if err := l.syncEndpoints(); err != nil {
		return nil, err
	}

	// Link the frontend and the upstream
	up.addFrontend(l.loc.GetUniqueId(), l)

	return l, nil
}

func (l *frontend) syncEndpoints() error {
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

func (l *frontend) upsertMiddleware(mi *backend.MiddlewareInstance) error {
	instance, err := mi.Middleware.NewMiddleware()
	if err != nil {
		return err
	}
	l.hloc.GetMiddlewareChain().Upsert(fmt.Sprintf("%s.%s", mi.Type, mi.Id), mi.Priority, instance)
	return nil
}

func (l *frontend) deleteMiddleware(mType, mId string) error {
	return l.hloc.GetMiddlewareChain().Remove(fmt.Sprintf("%s.%s", mType, mId))
}

func (l *frontend) updateOptions(loc *backend.Frontend) error {
	l.loc = *loc
	options, err := loc.GetOptions()
	if err != nil {
		return err
	}
	options.Transport = l.up.t
	return l.hloc.SetOptions(*options)
}

func (l *frontend) remove() error {
	router := l.m.getRouter(l.loc.Hostname)
	if router == nil {
		return fmt.Errorf("router for %s not found", l.loc.Hostname)
	}
	l.m.perfMon.deleteFrontend(l.loc.GetUniqueId())
	l.up.deleteFrontend(l.loc.GetUniqueId())
	return router.RemoveFrontendByExpression(l.loc.Path)
}
