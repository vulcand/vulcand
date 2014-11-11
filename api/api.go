package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll"

	"github.com/mailgun/vulcand/anomaly"
	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/plugin"
)

type ProxyController struct {
	backend backend.Backend
	stats   backend.StatsProvider
	app     *scroll.App
}

func InitProxyController(backend backend.Backend, stats backend.StatsProvider, app *scroll.App) {
	c := &ProxyController{backend: backend, stats: stats, app: app}

	app.SetNotFoundHandler(c.handleError)

	app.AddHandler(scroll.Spec{Path: "/v1/status", Methods: []string{"GET"}, HandlerWithBody: c.getStatus})

	app.AddHandler(scroll.Spec{Path: "/v1/log/severity", Methods: []string{"GET"}, Handler: c.getLogSeverity})
	app.AddHandler(scroll.Spec{Path: "/v1/log/severity", Methods: []string{"PUT"}, Handler: c.updateLogSeverity})

	app.AddHandler(scroll.Spec{Path: "/v1/hosts", Methods: []string{"POST"}, HandlerWithBody: c.addHost})
	app.AddHandler(scroll.Spec{Path: "/v1/hosts", Methods: []string{"GET"}, HandlerWithBody: c.getHosts})
	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}", Methods: []string{"GET"}, Handler: c.getHost})
	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}", Methods: []string{"DELETE"}, Handler: c.deleteHost})

	app.AddHandler(scroll.Spec{Path: "/v1/hosts/top/locations", Methods: []string{"GET"}, Handler: c.getTopLocations})

	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/locations/{id}", Methods: []string{"GET"}, Handler: c.getHostLocation})

	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/keypair", Methods: []string{"PUT"}, HandlerWithBody: c.updateHostKeyPair})

	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/listeners", Methods: []string{"POST"}, HandlerWithBody: c.addHostListener})
	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/listeners/{id}", Methods: []string{"DELETE"}, Handler: c.deleteHostListener})

	app.AddHandler(scroll.Spec{Path: "/v1/upstreams", Methods: []string{"POST"}, HandlerWithBody: c.addUpstream})
	app.AddHandler(scroll.Spec{Path: "/v1/upstreams", Methods: []string{"GET"}, Handler: c.getUpstreams})

	app.AddHandler(scroll.Spec{Path: "/v1/upstreams/{id}", Methods: []string{"DELETE"}, Handler: c.deleteUpstream})
	app.AddHandler(scroll.Spec{Path: "/v1/upstreams/{id}", Methods: []string{"GET"}, Handler: c.getUpstream})
	app.AddHandler(scroll.Spec{Path: "/v1/upstreams/{id}/options", Methods: []string{"PUT"}, HandlerWithBody: c.updateUpstreamOptions})

	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/locations", Methods: []string{"POST"}, HandlerWithBody: c.addLocation})
	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/locations", Methods: []string{"GET"}, Handler: c.getHostLocations})
	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/locations/{id}", Methods: []string{"PUT"}, Handler: c.updateLocationUpstream})

	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/locations/{id}/options", Methods: []string{"PUT"}, HandlerWithBody: c.updateLocationOptions})
	app.AddHandler(scroll.Spec{Path: "/v1/hosts/{hostname}/locations/{id}", Methods: []string{"DELETE"}, Handler: c.deleteLocation})

	app.AddHandler(scroll.Spec{Path: "/v1/upstreams/top/endpoints", Methods: []string{"GET"}, Handler: c.getTopEndpoints})
	app.AddHandler(scroll.Spec{Path: "/v1/upstreams/{upstream}/endpoints", Methods: []string{"POST"}, HandlerWithBody: c.addEndpoint})
	app.AddHandler(scroll.Spec{Path: "/v1/upstreams/{upstream}/endpoints", Methods: []string{"GET"}, Handler: c.getUpstreamEndpoints})
	app.AddHandler(scroll.Spec{Path: "/v1/upstreams/{upstream}/endpoints/{endpoint}", Methods: []string{"DELETE"}, Handler: c.deleteEndpoint})

	// Register controllers for middlewares
	if backend.GetRegistry() != nil {
		for _, s := range backend.GetRegistry().GetSpecs() {
			c.registerMiddlewareHandlers(s)
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

func (c *ProxyController) getLogSeverity(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	return scroll.Response{
		"severity": log.GetSeverity().String(),
	}, nil
}

func (c *ProxyController) updateLogSeverity(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	s, err := log.SeverityFromString(r.Form.Get("severity"))
	if err != nil {
		return nil, formatError(err)
	}
	log.SetSeverity(s)
	return scroll.Response{"message": fmt.Sprintf("Severity has been updated to %v", s)}, nil
}

func (c *ProxyController) getHosts(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	hosts, err := c.backend.GetHosts()

	// This is to display the realtime stats, looks ugly.
	for _, h := range hosts {
		for _, l := range h.Locations {
			if s, err := c.stats.GetLocationStats(l); err == nil {
				l.Stats = *s
			}
			for _, e := range l.Upstream.Endpoints {
				if s, err := c.stats.GetEndpointStats(e); err == nil {
					e.Stats = *s
				}
			}
		}
	}
	return scroll.Response{
		"Hosts": hosts,
	}, err
}

func (c *ProxyController) getHost(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	h, err := c.backend.GetHost(params["hostname"])
	if err != nil {
		return nil, formatError(err)
	}
	return formatResult(h, err)
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

func (c *ProxyController) getTopLocations(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	limit, err := strconv.Atoi(r.Form.Get("limit"))
	if err != nil {
		return nil, formatError(err)
	}
	locations, err := c.stats.GetTopLocations(r.Form.Get("hostname"), r.Form.Get("upstreamId"))
	if err != nil {
		return nil, formatError(err)
	}
	if limit > 0 && limit < len(locations) {
		locations = locations[:limit]
	}
	return scroll.Response{
		"Locations": locations,
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

func (c *ProxyController) updateUpstreamOptions(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	upId := params["id"]

	options, err := backend.UpstreamOptionsFromJSON(body)
	if err != nil {
		return nil, formatError(err)
	}
	return formatResult(c.backend.UpdateUpstreamOptions(upId, *options))
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

	for _, u := range upstreams {
		for _, e := range u.Endpoints {
			if s, err := c.stats.GetEndpointStats(e); err == nil {
				e.Stats = *s
			}
		}
		anomaly.MarkEndpointAnomalies(u.Endpoints)
	}

	return scroll.Response{
		"Upstreams": upstreams,
	}, err
}

func (c *ProxyController) getUpstreamEndpoints(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	up, err := c.backend.GetUpstream(params["upstream"])
	if err != nil {
		return nil, formatError(err)
	}
	for _, e := range up.Endpoints {
		if s, err := c.stats.GetEndpointStats(e); err == nil {
			e.Stats = *s
		}
	}
	anomaly.MarkEndpointAnomalies(up.Endpoints)
	return scroll.Response{
		"Endpoints": up.Endpoints,
	}, nil
}

func (c *ProxyController) getTopEndpoints(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	limit, err := strconv.Atoi(r.Form.Get("limit"))
	if err != nil {
		return nil, formatError(err)
	}
	endpoints, err := c.stats.GetTopEndpoints(r.Form.Get("upstreamId"))
	if err != nil {
		return nil, formatError(err)
	}
	if r.Form.Get("upstreamId") != "" {
		anomaly.MarkEndpointAnomalies(endpoints)
	}
	if limit > 0 && limit < len(endpoints) {
		endpoints = endpoints[:limit]
	}
	return scroll.Response{
		"Endpoints": endpoints,
	}, nil
}

func (c *ProxyController) getUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	return formatResult(c.backend.GetUpstream(params["id"]))
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
	c.app.AddHandler(
		scroll.Spec{
			Path:            fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s", spec.Type),
			Methods:         []string{"POST"},
			HandlerWithBody: c.makeAddMiddleware(spec),
		})

	c.app.AddHandler(
		scroll.Spec{
			Path:    fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
			Methods: []string{"GET"},
			Handler: c.makeGetMiddleware(spec),
		})

	c.app.AddHandler(
		scroll.Spec{
			Path:            fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
			Methods:         []string{"PUT"},
			HandlerWithBody: c.makeUpdateMiddleware(spec),
		})

	c.app.AddHandler(
		scroll.Spec{
			Path:    fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
			Methods: []string{"DELETE"},
			Handler: c.makeDeleteMiddleware(spec),
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
