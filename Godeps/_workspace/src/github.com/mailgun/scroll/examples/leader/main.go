package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/scroll/registry"
)

var host string
var port int

func init() {
	flag.StringVar(&host, "host", "0.0.0.0", "")
	flag.IntVar(&port, "port", 9000, "")
	flag.Parse()
}

func main() {
	name := "leader"

	appConfig := scroll.AppConfig{
		Name:             name,
		ListenIP:         host,
		ListenPort:       port,
		Registry:         registry.NewLeaderRegistry("scrollexamples/leader", "master", 5),
		PublicAPIHost:    "public.local",
		ProtectedAPIHost: "private.local",
	}

	handlerSpec := scroll.Spec{
		Scopes:  []scroll.Scope{scroll.ScopePublic, scroll.ScopeProtected},
		Methods: []string{"GET"},
		Paths:   []string{"/"},
		Handler: index,
	}

	fmt.Printf("Starting %s on %s:%d...\n", name, host, port)

	app := scroll.NewAppWithConfig(appConfig)
	app.AddHandler(handlerSpec)
	app.Run()
}

func index(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	message := fmt.Sprintf("Running on %s:%d", host, port)
	return scroll.Response{"message": message}, nil
}
