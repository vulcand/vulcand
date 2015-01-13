// This file will be generated to include all customer specific middlewares
package registry

import (
	"github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/plugin/cbreaker"
	"github.com/mailgun/vulcand/plugin/connlimit"
	"github.com/mailgun/vulcand/plugin/ratelimit"
	"github.com/mailgun/vulcand/plugin/rewrite"
	"github.com/mailgun/vulcand/plugin/trace"
)

func GetRegistry() *plugin.Registry {
	r := plugin.NewRegistry()

	if err := r.AddSpec(ratelimit.GetSpec()); err != nil {
		panic(err)
	}

	if err := r.AddSpec(connlimit.GetSpec()); err != nil {
		panic(err)
	}

	if err := r.AddSpec(rewrite.GetSpec()); err != nil {
		panic(err)
	}

	if err := r.AddSpec(cbreaker.GetSpec()); err != nil {
		panic(err)
	}

	if err := r.AddSpec(trace.GetSpec()); err != nil {
		panic(err)
	}

	return r
}
