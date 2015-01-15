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

	specs := []*plugin.MiddlewareSpec{
		ratelimit.GetSpec(),
		connlimit.GetSpec(),
		rewrite.GetSpec(),
		cbreaker.GetSpec(),
		trace.GetSpec(),
	}

	for _, spec := range specs {
		if err := r.AddSpec(spec); err != nil {
			panic(err)
		}
	}

	return r
}
