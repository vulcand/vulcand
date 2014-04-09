package api

import (
	"fmt"
	"github.com/gorilla/mux"
	api "github.com/mailgun/gotools-api"
	log "github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcand/backend"
	"net/http"
)

type ProxyController struct {
	backend backend.Backend
}

func InitProxyController(backend backend.Backend, router *mux.Router) {
	controller := &ProxyController{backend: backend}

	router.HandleFunc("/v1/hosts", api.MakeHandler(controller.GetHosts)).Methods("GET")
	router.HandleFunc("/v1/hosts", api.MakeHandler(controller.AddHost)).Methods("POST")
	router.HandleFunc("/v1/hosts/{hostname}", api.MakeHandler(controller.DeleteHost)).Methods("DELETE")
	router.HandleFunc("/v1/hosts/{hostname}/locations", api.MakeHandler(controller.AddLocation)).Methods("POST")
	router.HandleFunc("/v1/hosts/{hostname}/locations/{id}", api.MakeHandler(controller.DeleteLocation)).Methods("DELETE")
	router.HandleFunc("/v1/hosts/{hostname}/locations/{id}", api.MakeHandler(controller.UpdateLocation)).Methods("PUT")
	router.HandleFunc("/v1/upstreams", api.MakeHandler(controller.AddUpstream)).Methods("POST")
	router.HandleFunc("/v1/upstreams", api.MakeHandler(controller.GetUpstreams)).Methods("GET")
	router.HandleFunc("/v1/upstreams/{id}", api.MakeHandler(controller.DeleteUpstream)).Methods("DELETE")
	router.HandleFunc("/v1/upstreams/{upstream}/endpoints", api.MakeHandler(controller.AddEndpoint)).Methods("POST")
	router.HandleFunc("/v1/upstreams/{upstream}/endpoints/{endpoint}", api.MakeHandler(controller.DeleteEndpoint)).Methods("DELETE")
}

func (c *ProxyController) GetHosts(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	servers, err := c.backend.GetHosts()
	return api.Response{
		"Hosts": servers,
	}, err
}

func (c *ProxyController) AddHost(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	name, err := api.GetStringField(r, "name")
	if err != nil {
		return nil, err
	}
	log.Infof("Add host: %s", name)
	if err := c.backend.AddHost(name); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}

	return api.Response{"message": "Host added"}, nil
}

func (c *ProxyController) DeleteHost(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	log.Infof("Delete host: %s", params["hostname"])
	if err := c.backend.DeleteHost(params["hostname"]); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}
	return api.Response{"message": "Host deleted"}, nil
}

func (c *ProxyController) AddLocation(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	hostname := params["hostname"]

	id, err := api.GetStringField(r, "id")
	if err != nil {
		return nil, err
	}

	path, err := api.GetStringField(r, "path")
	if err != nil {
		return nil, err
	}
	upstream, err := api.GetStringField(r, "upstream")
	if err != nil {
		return nil, err
	}

	log.Infof("Add Location: %s %s", hostname, path)
	if err := c.backend.AddLocation(id, hostname, path, upstream); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}

	return api.Response{"message": "Location added"}, nil
}

func (c *ProxyController) UpdateLocation(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	hostname := params["hostname"]
	locationId := params["id"]

	upstream, err := api.GetStringField(r, "upstream")
	if err != nil {
		return nil, err
	}

	log.Infof("Update Location: %s %s set upstream", hostname, locationId, upstream)
	if err := c.backend.UpdateLocationUpstream(hostname, locationId, upstream); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}

	return api.Response{"message": "Location upstream updated"}, nil
}

func (c *ProxyController) DeleteLocation(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	log.Infof("Delete Location(id=%s) from Host(name=%s)", params["id"], params["hostname"])
	if err := c.backend.DeleteLocation(params["hostname"], params["id"]); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}
	return api.Response{"message": "Location deleted"}, nil
}

func (c *ProxyController) GetUpstreams(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	upstreams, err := c.backend.GetUpstreams()
	return api.Response{
		"Upstreams": upstreams,
	}, err
}

func (c *ProxyController) AddUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	id, err := api.GetStringField(r, "id")
	if err != nil {
		return nil, err
	}
	log.Infof("Add Upstream: %s", id)
	if err := c.backend.AddUpstream(id); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}
	return api.Response{"message": "Upstream added"}, nil
}

func (c *ProxyController) DeleteUpstream(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	log.Infof("Delete Upstream(id=%s)", params["upstream"])
	if err := c.backend.DeleteUpstream(params["upstream"]); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}
	return api.Response{"message": "Upstream deleted"}, nil
}

func (c *ProxyController) AddEndpoint(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	url, err := api.GetStringField(r, "url")
	if err != nil {
		return nil, err
	}
	id, err := api.GetStringField(r, "id")
	if err != nil {
		return nil, err
	}

	upstreamId := params["upstream"]
	log.Infof("Add Endpoint %s to %s", url, upstreamId)

	if err := c.backend.AddEndpoint(upstreamId, id, url); err != nil {
		return nil, api.GenericAPIError{Reason: fmt.Sprintf("%s", err)}
	}
	return api.Response{"message": "Endpoint added"}, nil
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
