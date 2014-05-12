// This file will be generated to include all customer specific middlewares
package registry

import (
	. "github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/plugin/connlimit"
	"github.com/mailgun/vulcand/plugin/ratelimit"
)

func GetRegistry() *Registry {
	r := NewRegistry()

	if err := r.AddSpec(ratelimit.GetSpec()); err != nil {
		panic(err)
	}

	if err := r.AddSpec(connlimit.GetSpec()); err != nil {
		panic(err)
	}

	return r
}
