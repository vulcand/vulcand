package api

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"

	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/connwatch"
	"github.com/mailgun/vulcand/plugin"
)

type ProxyController struct {
	backend     backend.Backend
	connWatcher *connwatch.ConnectionWatcher
	statsGetter backend.StatsGetter
	app         *scroll.App
}

func InitProxyController(backend backend.Backend, statsGetter backend.StatsGetter, connWatcher *connwatch.ConnectionWatcher, app *scroll.App) {
	c := &ProxyController{backend: backend, statsGetter: statsGetter, connWatcher: connWatcher, app: app}

	app.SetNotFoundHandler(c.handleError)

	app.AddHandlerWithBody(c.getStatus, &scroll.HandlerConfig{Path: "/v1/status", Methods: []string{"GET"}})
	app.AddHandlerWithBody(c.getHosts, &scroll.HandlerConfig{Path: "/v1/hosts", Methods: []string{"GET"}})
	app.AddHandlerWithBody(c.addHost, &scroll.HandlerConfig{Path: "/v1/hosts", Methods: []string{"POST"}})

	app.AddHandler(c.getHostLocation, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/locations/{id}", Methods: []string{"GET"}})
	app.AddHandler(c.deleteHost, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}", Methods: []string{"DELETE"}})

	app.AddHandler(c.deleteHost, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}", Methods: []string{"DELETE"}})
	app.AddHandlerWithBody(c.updateHostKeyPair, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/keypair", Methods: []string{"PUT"}})
	app.AddHandlerWithBody(c.addHostListener, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/listeners", Methods: []string{"POST"}})
	app.AddHandler(c.deleteHostListener, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/listeners/{id}", Methods: []string{"DELETE"}})

	///
	app.AddHandlerWithBody(c.addUpstream, &scroll.HandlerConfig{Path: "/v1/upstreams", Methods: []string{"POST"}})
	app.AddHandler(c.getUpstreams, &scroll.HandlerConfig{Path: "/v1/upstreams", Methods: []string{"GET"}})

	app.AddHandler(c.deleteUpstream, &scroll.HandlerConfig{Path: "/v1/upstreams/{id}", Methods: []string{"DELETE"}})
	app.AddHandler(c.getUpstream, &scroll.HandlerConfig{Path: "/v1/upstreams/{id}", Methods: []string{"GET"}})
	app.AddHandler(c.drainUpstreamConnections, &scroll.HandlerConfig{Path: "/v1/upstreams/{id}/drain", Methods: []string{"GET"}})

	app.AddHandlerWithBody(c.addLocation, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/locations", Methods: []string{"POST"}})
	app.AddHandler(c.getHostLocations, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/locations", Methods: []string{"GET"}})
	app.AddHandler(c.updateLocationUpstream, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/locations/{id}", Methods: []string{"PUT"}})

	app.AddHandlerWithBody(c.updateLocationOptions, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/locations/{id}/options", Methods: []string{"PUT"}})
	app.AddHandler(c.deleteLocation, &scroll.HandlerConfig{Path: "/v1/hosts/{hostname}/locations/{id}", Methods: []string{"DELETE"}})

	app.AddHandlerWithBody(c.addEndpoint, &scroll.HandlerConfig{Path: "/v1/upstreams/{upstream}/endpoints", Methods: []string{"POST"}})
	app.AddHandler(c.getUpstreamEndpoints, &scroll.HandlerConfig{Path: "/v1/upstreams/{upstream}/endpoints", Methods: []string{"GET"}})
	app.AddHandler(c.deleteEndpoint, &scroll.HandlerConfig{Path: "/v1/upstreams/{upstream}/endpoints/{endpoint}", Methods: []string{"DELETE"}})

	// Register controllers for middlewares
	if backend.GetRegistry() != nil {
		for _, middlewareSpec := range backend.GetRegistry().GetSpecs() {
			c.registerMiddlewareHandlers(middlewareSpec)
		}
	}
}

func (c *ProxyController) handleError(w http.ResponseWriter, r *http.Request) {
	scroll.ReplyError(w, scroll.NotFoundError{Description: "Object not found"})
}

func (c *ProxyController) getStatus(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	return scroll.Response{
		"Status": "ok",
	}, nil
}

func (c *ProxyController) getHosts(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	hosts, err := c.backend.GetHosts()

	// This is to display the realtime stats, looks ugly.
	for _, h := range hosts {
		for _, l := range h.Locations {
			for _, e := range l.Upstream.Endpoints {
				fmt.Printf("Endpoint Stats: %s\n", l)
				e.Stats = c.statsGetter.GetStats(h.Name, l.Id, e)
				fmt.Printf("Endpoint Stats: %s stats: %s\n", e, e.Stats)
			}
		}
	}
	return scroll.Response{
		"Hosts": hosts,
	}, err
}

func (c *ProxyController) getHostLocations(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	host, err := c.backend.GetHost(params["hostname"])
	if err != nil {
		return nil, formatError(err)
	}
	return scroll.Response{
		"Locations": host.Locations,
	}, nil
}

func (c *ProxyController) getHostLocation(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	return formatResult(c.backend.GetLocation(params["hostname"], params["id"]))
}

func (c *ProxyController) addHost(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	host, err := backend.HostFromJSON(body, c.backend.GetRegistry().GetSpec)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add %s", host)
	return formatResult(c.backend.AddHost(host))
}

func (c *ProxyController) addHostListener(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	listener, err := backend.ListenerFromJSON(body)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add %s", listener)
	return formatResult(c.backend.AddHostListener(params["hostname"], listener))
}

func (c *ProxyController) deleteHostListener(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	log.Infof("Delete Listener(id=%s) from Host(name=%s)", params["id"], params["hostname"])
	if err := c.backend.DeleteHostListener(params["hostname"], params["id"]); err != nil {
		return nil, formatError(err)
	}
	return scroll.Response{"message": "Listener deleted"}, nil
}

func (c *ProxyController) updateHostKeyPair(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	hostname := params["hostname"]
	keyPair, err := backend.KeyPairFromJSON(body)
	if err != nil {
		return nil, formatError(err)
	}
	return formatResult(c.backend.UpdateHostKeyPair(hostname, keyPair))
}

func (c *ProxyController) deleteHost(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	hostname := params["hostname"]
	log.Infof("Delete host: %s", hostname)
	if err := c.backend.DeleteHost(hostname); err != nil {
		return nil, formatError(err)
	}
	return scroll.Response{"message": fmt.Sprintf("Host '%s' deleted", hostname)}, nil
}

func (c *ProxyController) addUpstream(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	upstream, err := backend.UpstreamFromJSON(body)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add Upstream: %s", upstream)
	return formatResult(c.backend.AddUpstream(upstream))
}

func (c *ProxyController) deleteUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstreamId := params["id"]
	log.Infof("Delete Upstream(id=%s)", upstreamId)
	if err := c.backend.DeleteUpstream(upstreamId); err != nil {
		return nil, formatError(err)
	}
	return scroll.Response{"message": "Upstream deleted"}, nil
}

func (c *ProxyController) getUpstreams(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstreams, err := c.backend.GetUpstreams()
	return scroll.Response{
		"Upstreams": upstreams,
	}, err
}

func (c *ProxyController) getUpstreamEndpoints(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	up, err := c.backend.GetUpstream(params["upstream"])
	if err != nil {
		return nil, formatError(err)
	}
	return scroll.Response{
		"Endpoints": up.Endpoints,
	}, nil
}

func (c *ProxyController) getUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	return formatResult(c.backend.GetUpstream(params["id"]))
}

func (c *ProxyController) drainUpstreamConnections(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstream, err := c.backend.GetUpstream(params["id"])
	if err != nil {
		return nil, formatError(err)
	}
	timeout, err := scroll.GetIntField(r, "timeout")
	if err != nil {
		return nil, formatError(err)
	}

	endpoints := make([]*url.URL, len(upstream.Endpoints))
	for i, e := range upstream.Endpoints {
		u, err := netutils.ParseUrl(e.Url)
		if err != nil {
			return nil, err
		}
		endpoints[i] = u
	}

	connections, err := c.connWatcher.DrainConnections(time.Duration(timeout)*time.Second, endpoints...)
	if err != nil {
		return nil, err
	}
	return scroll.Response{
		"Connections": connections,
	}, err
}

func (c *ProxyController) addLocation(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	location, err := backend.LocationFromJSON(body, c.backend.GetRegistry().GetSpec)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add %s", location)
	return formatResult(c.backend.AddLocation(location))
}

func (c *ProxyController) updateLocationUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	hostname := params["hostname"]
	locationId := params["id"]

	upstream, err := scroll.GetStringField(r, "upstream")
	if err != nil {
		return nil, err
	}

	log.Infof("Update Location: %s %s set upstream", hostname, locationId, upstream)
	if _, err := c.backend.UpdateLocationUpstream(hostname, locationId, upstream); err != nil {
		return nil, formatError(err)
	}
	return scroll.Response{"message": "Location upstream updated"}, nil
}

func (c *ProxyController) updateLocationOptions(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	hostname := params["hostname"]
	locationId := params["id"]

	options, err := backend.LocationOptionsFromJSON(body)
	if err != nil {
		return nil, formatError(err)
	}
	return formatResult(c.backend.UpdateLocationOptions(hostname, locationId, *options))
}

func (c *ProxyController) deleteLocation(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	log.Infof("Delete Location(id=%s) from Host(name=%s)", params["id"], params["hostname"])
	if err := c.backend.DeleteLocation(params["hostname"], params["id"]); err != nil {
		return nil, formatError(err)
	}
	return scroll.Response{"message": "Location deleted"}, nil
}

func (c *ProxyController) addEndpoint(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	upstreamId := params["upstream"]
	ep, err := backend.EndpointFromJSON(body)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add %s to %s", ep, upstreamId)
	return formatResult(c.backend.AddEndpoint(ep))
}

func (c *ProxyController) deleteEndpoint(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstreamId := params["upstream"]
	id := params["endpoint"]

	log.Infof("Delete Endpoint(url=%s) from Upstream(id=%s)", id, upstreamId)
	if err := c.backend.DeleteEndpoint(upstreamId, id); err != nil {
		return nil, scroll.GenericAPIError{Reason: err.Error()}
	}
	return scroll.Response{"message": "Endpoint deleted"}, nil
}

func (c *ProxyController) registerMiddlewareHandlers(spec *plugin.MiddlewareSpec) {
	c.app.AddHandlerWithBody(
		c.makeAddMiddleware(spec),
		&scroll.HandlerConfig{
			Path:    fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s", spec.Type),
			Methods: []string{"POST"},
		})

	c.app.AddHandler(
		c.makeGetMiddleware(spec),
		&scroll.HandlerConfig{
			Path:    fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
			Methods: []string{"GET"},
		})

	c.app.AddHandlerWithBody(
		c.makeUpdateMiddleware(spec),
		&scroll.HandlerConfig{
			Path:    fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
			Methods: []string{"PUT"},
		})

	c.app.AddHandler(
		c.makeDeleteMiddleware(spec),
		&scroll.HandlerConfig{
			Path:    fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
			Methods: []string{"DELETE"},
		})
}

func (c *ProxyController) makeAddMiddleware(spec *plugin.MiddlewareSpec) scroll.HandlerWithBodyFunc {
	return func(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
		hostname := params["hostname"]
		location := params["location"]
		m, err := backend.MiddlewareFromJSON(body, c.backend.GetRegistry().GetSpec)
		if err != nil {
			return nil, formatError(err)
		}
		return formatResult(c.backend.AddLocationMiddleware(hostname, location, m))
	}
}

func (c *ProxyController) makeUpdateMiddleware(spec *plugin.MiddlewareSpec) scroll.HandlerWithBodyFunc {
	return func(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
		hostname := params["hostname"]
		location := params["location"]
		m, err := backend.MiddlewareFromJSON(body, c.backend.GetRegistry().GetSpec)
		if err != nil {
			return nil, formatError(err)
		}
		return formatResult(c.backend.UpdateLocationMiddleware(hostname, location, m))
	}
}

func (c *ProxyController) makeGetMiddleware(spec *plugin.MiddlewareSpec) scroll.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
		return formatResult(c.backend.GetLocationMiddleware(params["hostname"], params["location"], spec.Type, params["id"]))
	}
}

func (c *ProxyController) makeDeleteMiddleware(spec *plugin.MiddlewareSpec) scroll.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
		hostname := params["hostname"]
		location := params["location"]
		mId := params["id"]
		if err := c.backend.DeleteLocationMiddleware(hostname, location, spec.Type, mId); err != nil {
			return nil, formatError(err)
		}
		return scroll.Response{"message": "Middleware deleted"}, nil
	}
}

func formatError(e error) error {
	switch err := e.(type) {
	case *backend.AlreadyExistsError:
		return scroll.ConflictError{Description: err.Error()}
	case *backend.NotFoundError:
		return scroll.NotFoundError{Description: err.Error()}
	}
	return scroll.GenericAPIError{Reason: e.Error()}
}

func formatResult(in interface{}, err error) (interface{}, error) {
	if err != nil {
		return nil, formatError(err)
	}
	return in, nil
}
