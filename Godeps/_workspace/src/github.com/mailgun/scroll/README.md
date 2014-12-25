**WORK IN PROGRESS**

scroll
======

Scroll is a lightweight library for building Go HTTP services at Mailgun.

Example
-------

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/mailgun/scroll"
)

func handler(w http.ResponseWriter, r *http.Request, params map[string]string) (interface{}, error) {
	return scroll.Response{
		"message": fmt.Sprintf("Resource ID: %v", params["resourceID"]),
	}, nil
}

func main() {
	// create an app
	appConfig := scroll.AppConfig{
		Name:       "scrollexample",
		ListenIP:   "0.0.0.0",
		ListenPort: 8080,
		Register:   false,
	}
	app := scroll.NewAppWithConfig(appConfig)

	// register a handler
	handlerSpec := scroll.Spec{

		Methods:  []string{"GET", "POST"},
		Path:     "/resources/{resourceID}",
		Register: false,
		Handler:  handler,
	}

	app.AddHandler(handlerSpec)

	// start the app
	app.Run()
}
```