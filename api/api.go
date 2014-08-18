package api

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	api "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-api"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/gorilla/mux"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"

	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/connwatch"
	"github.com/mailgun/vulcand/plugin"
)

type ProxyController struct {
	backend     Backend
	connWatcher *ConnectionWatcher
	statsGetter StatsGetter
}

func InitProxyController(backend Backend, statsGetter StatsGetter, connWatcher *ConnectionWatcher, router *mux.Router) {
	controller := &ProxyController{backend: backend, statsGetter: statsGetter, connWatcher: connWatcher}

	router.NotFoundHandler = api.MakeRawHandler(controller.HandleError)
	router.HandleFunc("/v1/status", api.MakeRawHandler(controller.GetStatus)).Methods("GET")
	router.HandleFunc("/v1/hosts", api.MakeRawHandler(controller.GetHosts)).Methods("GET")
	router.HandleFunc("/v1/hosts", api.MakeRawHandler(controller.AddHost)).Methods("POST")
	router.HandleFunc("/v1/hosts/{hostname}/locations/{id}", api.MakeHandler(controller.GetHostLocation)).Methods("GET")
	router.HandleFunc("/v1/hosts/{hostname}", api.MakeHandler(controller.DeleteHost)).Methods("DELETE")

	router.HandleFunc("/v1/upstreams", api.MakeRawHandler(controller.AddUpstream)).Methods("POST")
	router.HandleFunc("/v1/upstreams", api.MakeHandler(controller.GetUpstreams)).Methods("GET")
	router.HandleFunc("/v1/upstreams/{id}", api.MakeHandler(controller.DeleteUpstream)).Methods("DELETE")
	router.HandleFunc("/v1/upstreams/{id}", api.MakeHandler(controller.GetUpstream)).Methods("GET")
	router.HandleFunc("/v1/upstreams/{id}/drain", api.MakeHandler(controller.DrainUpstreamConnections)).Methods("GET")

	router.HandleFunc("/v1/hosts/{hostname}/locations", api.MakeRawHandler(controller.AddLocation)).Methods("POST")
	router.HandleFunc("/v1/hosts/{hostname}/locations", api.MakeHandler(controller.GetHostLocations)).Methods("GET")
	router.HandleFunc("/v1/hosts/{hostname}/locations/{id}", api.MakeHandler(controller.UpdateLocationUpstream)).Methods("PUT")
	router.HandleFunc("/v1/hosts/{hostname}/locations/{id}/options", api.MakeRawHandler(controller.UpdateLocationOptions)).Methods("PUT")
	router.HandleFunc("/v1/hosts/{hostname}/locations/{id}", api.MakeHandler(controller.DeleteLocation)).Methods("DELETE")

	router.HandleFunc("/v1/upstreams/{upstream}/endpoints", api.MakeRawHandler(controller.AddEndpoint)).Methods("POST")
	router.HandleFunc("/v1/upstreams/{upstream}/endpoints", api.MakeHandler(controller.GetUpstreamEndpoints)).Methods("GET")
	router.HandleFunc("/v1/upstreams/{upstream}/endpoints/{endpoint}", api.MakeHandler(controller.DeleteEndpoint)).Methods("DELETE")

	// Register controllers for middlewares
	if backend.GetRegistry() != nil {
		for _, middlewareSpec := range backend.GetRegistry().GetSpecs() {
			controller.registerMiddlewareHandlers(router, middlewareSpec)
		}
	}
}

func (c *ProxyController) HandleError(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	return nil, api.NotFoundError{Description: "Object not found"}
}

func (c *ProxyController) GetStatus(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	return api.Response{
		"Status": "ok",
	}, nil
}

func (c *ProxyController) GetHosts(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
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
	return api.Response{
		"Hosts": hosts,
	}, err
}

func (c *ProxyController) GetHostLocations(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	host, err := c.backend.GetHost(params["hostname"])
	if err != nil {
		return nil, formatError(err)
	}
	return api.Response{
		"Locations": host.Locations,
	}, nil
}

func (c *ProxyController) GetHostLocation(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	return formatResult(c.backend.GetLocation(params["hostname"], params["id"]))
}

func (c *ProxyController) AddHost(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	host, err := HostFromJson(body, c.backend.GetRegistry().GetSpec)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add %s", host)
	return formatResult(c.backend.AddHost(host))
}

func (c *ProxyController) DeleteHost(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	hostname := params["hostname"]
	log.Infof("Delete host: %s", hostname)
	if err := c.backend.DeleteHost(hostname); err != nil {
		return nil, formatError(err)
	}
	return api.Response{"message": fmt.Sprintf("Host '%s' deleted", hostname)}, nil
}

func (c *ProxyController) AddUpstream(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	upstream, err := UpstreamFromJson(body)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add Upstream: %s", upstream)
	return formatResult(c.backend.AddUpstream(upstream))
}

func (c *ProxyController) DeleteUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstreamId := params["id"]
	log.Infof("Delete Upstream(id=%s)", upstreamId)
	if err := c.backend.DeleteUpstream(upstreamId); err != nil {
		return nil, formatError(err)
	}
	return api.Response{"message": "Upstream deleted"}, nil
}

func (c *ProxyController) GetUpstreams(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstreams, err := c.backend.GetUpstreams()
	return api.Response{
		"Upstreams": upstreams,
	}, err
}

func (c *ProxyController) GetUpstreamEndpoints(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	up, err := c.backend.GetUpstream(params["upstream"])
	if err != nil {
		return nil, formatError(err)
	}
	return api.Response{
		"Endpoints": up.Endpoints,
	}, nil
}

func (c *ProxyController) GetUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	return formatResult(c.backend.GetUpstream(params["id"]))
}

func (c *ProxyController) DrainUpstreamConnections(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstream, err := c.backend.GetUpstream(params["id"])
	if err != nil {
		return nil, formatError(err)
	}
	timeout, err := api.GetIntField(r, "timeout")
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
	return api.Response{
		"Connections": connections,
	}, err
}

func (c *ProxyController) AddLocation(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	location, err := LocationFromJson(body, c.backend.GetRegistry().GetSpec)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add %s", location)
	return formatResult(c.backend.AddLocation(location))
}

func (c *ProxyController) UpdateLocationUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	hostname := params["hostname"]
	locationId := params["id"]

	upstream, err := api.GetStringField(r, "upstream")
	if err != nil {
		return nil, err
	}

	log.Infof("Update Location: %s %s set upstream", hostname, locationId, upstream)
	if _, err := c.backend.UpdateLocationUpstream(hostname, locationId, upstream); err != nil {
		return nil, formatError(err)
	}
	return api.Response{"message": "Location upstream updated"}, nil
}

func (c *ProxyController) UpdateLocationOptions(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	hostname := params["hostname"]
	locationId := params["id"]

	options, err := LocationOptionsFromJson(body)
	if err != nil {
		return nil, formatError(err)
	}
	return formatResult(c.backend.UpdateLocationOptions(hostname, locationId, *options))
}

func (c *ProxyController) DeleteLocation(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	log.Infof("Delete Location(id=%s) from Host(name=%s)", params["id"], params["hostname"])
	if err := c.backend.DeleteLocation(params["hostname"], params["id"]); err != nil {
		return nil, formatError(err)
	}
	return api.Response{"message": "Location deleted"}, nil
}

func (c *ProxyController) AddEndpoint(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
	upstreamId := params["upstream"]
	ep, err := EndpointFromJson(body)
	if err != nil {
		return nil, formatError(err)
	}
	log.Infof("Add %s to %s", ep, upstreamId)
	return formatResult(c.backend.AddEndpoint(ep))
}

func (c *ProxyController) DeleteEndpoint(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstreamId := params["upstream"]
	id := params["endpoint"]

	log.Infof("Delete Endpoint(url=%s) from Upstream(id=%s)", id, upstreamId)
	if err := c.backend.DeleteEndpoint(upstreamId, id); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}
	return api.Response{"message": "Endpoint deleted"}, nil
}

func (c *ProxyController) registerMiddlewareHandlers(router *mux.Router, spec *plugin.MiddlewareSpec) {
	router.HandleFunc(
		fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s", spec.Type),
		c.makeAddMiddleware(spec)).Methods("POST")

	router.HandleFunc(
		fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
		c.makeGetMiddleware(spec)).Methods("GET")

	router.HandleFunc(
		fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
		c.makeUpdateMiddleware(spec)).Methods("PUT")

	router.HandleFunc(
		fmt.Sprintf("/v1/hosts/{hostname}/locations/{location}/middlewares/%s/{id}", spec.Type),
		c.makeDeleteMiddleware(spec)).Methods("DELETE")
}

func (c *ProxyController) makeAddMiddleware(spec *plugin.MiddlewareSpec) http.HandlerFunc {
	return api.MakeRawHandler(func(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
		hostname := params["hostname"]
		location := params["location"]
		m, err := MiddlewareFromJson(body, c.backend.GetRegistry().GetSpec)
		if err != nil {
			return nil, formatError(err)
		}
		return formatResult(c.backend.AddLocationMiddleware(hostname, location, m))
	})
}

func (c *ProxyController) makeUpdateMiddleware(spec *plugin.MiddlewareSpec) http.HandlerFunc {
	return api.MakeRawHandler(func(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
		hostname := params["hostname"]
		location := params["location"]
		m, err := MiddlewareFromJson(body, c.backend.GetRegistry().GetSpec)
		if err != nil {
			return nil, formatError(err)
		}
		return formatResult(c.backend.UpdateLocationMiddleware(hostname, location, m))
	})
}

func (c *ProxyController) makeGetMiddleware(spec *plugin.MiddlewareSpec) http.HandlerFunc {
	return api.MakeRawHandler(func(w http.ResponseWriter, r *http.Request, params map[string]string, body []byte) (interface{}, error) {
		return formatResult(c.backend.GetLocationMiddleware(params["hostname"], params["location"], spec.Type, params["id"]))
	})
}

func (c *ProxyController) makeDeleteMiddleware(spec *plugin.MiddlewareSpec) http.HandlerFunc {
	return api.MakeHandler(func(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
		hostname := params["hostname"]
		location := params["location"]
		mId := params["id"]
		if err := c.backend.DeleteLocationMiddleware(hostname, location, spec.Type, mId); err != nil {
			return nil, formatError(err)
		}
		return api.Response{"message": "Middleware deleted"}, nil
	})
}

func formatError(e error) error {
	switch err := e.(type) {
	case *AlreadyExistsError:
		return api.ConflictError{Description: fmt.Sprintf("%s", err)}
	case *NotFoundError:
		return api.NotFoundError{Description: fmt.Sprintf("%s", err)}
	}
	return api.GenericAPIError{Reason: fmt.Sprintf("%s", e)}
}

func formatResult(in interface{}, err error) (interface{}, error) {
	if err != nil {
		return nil, formatError(err)
	}
	return in, nil
}
