package builder

import (
	"github.com/vulcand/vulcand/proxy"
	"github.com/vulcand/vulcand/proxy/mux"
	"github.com/vulcand/vulcand/stapler"
)

// NewProxy returns a new Proxy instance.
func NewProxy(id int, st stapler.Stapler, o proxy.Options) (proxy.Proxy, error) {
	return mux.New(id, st, o)
}
