package api

import (
	"github.com/gorilla/mux"
	api "github.com/mailgun/gotools-api"
	log "github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcand/proxy"
	"net/http"
)

type ServerController struct {
	proxy proxy.Proxy
}

func InitServerController(proxy proxy.Proxy, router *mux.Router) {
	controller := &ServerController{proxy: proxy}

	router.HandleFunc("/v1/servers", api.MakeHandler(controller.Get)).Methods("GET")
}

func (c *ServerController) Get(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	log.Infof("Get servers")
	servers, err := c.proxy.GetServers()
	return api.Response{
		"Servers": servers,
	}, err
}

func (c *ServerController) Post(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	log.Infof("Create server")

	name, err := api.GetStringField(r, "name")
	if err != nil {
		return nil, err
	}

	if err := c.proxy.AddServer(name); err != nil {
		return nil, err
	}

	return api.Response{"status": "ok"}, nil
}
