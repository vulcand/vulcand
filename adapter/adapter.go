// Adapter contains the functions and objects to convert vulcan library specific interfaces that are more generic
// into vulcan daemon specific interfaces and data structures.
package adapter

import (
	"github.com/mailgun/vulcan"
	"github.com/mailgun/vulcan/limit/connlimit"
	"github.com/mailgun/vulcan/limit/tokenbucket"
	"github.com/mailgun/vulcan/loadbalance/roundrobin"
	"github.com/mailgun/vulcan/location/httploc"
	"github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcan/route/hostroute"
	"github.com/mailgun/vulcan/route/pathroute"
	. "github.com/mailgun/vulcand/backend"
	"time"
)

func NewRateLimiter(rl *RateLimit) (*tokenbucket.TokenLimiter, error) {
	mapper, err := VariableToMapper(rl.Variable)
	if err != nil {
		return nil, err
	}
	rate := tokenbucket.Rate{Units: int64(rl.Requests), Period: time.Second * time.Duration(rl.PeriodSeconds)}
	return tokenbucket.NewTokenLimiterWithOptions(mapper, rate, tokenbucket.Options{Burst: rl.Burst})
}

func NewConnLimiter(cl *ConnLimit) (*connlimit.ConnectionLimiter, error) {
	mapper, err := VariableToMapper(cl.Variable)
	if err != nil {
		return nil, err
	}
	return connlimit.NewConnectionLimiter(mapper, cl.Connections)
}

// Adapter helps to convert vulcan library-specific interfaces to vulcand interfaces and data structures
type Adapter struct {
	proxy *vulcan.Proxy
}

func NewAdapter(proxy *vulcan.Proxy) *Adapter {
	return &Adapter{
		proxy: proxy,
	}
}

func (a *Adapter) GetHostRouter() *hostroute.HostRouter {
	return a.proxy.GetRouter().(*hostroute.HostRouter)
}

func (a *Adapter) GetPathRouter(hostname string) *pathroute.PathRouter {
	r := a.GetHostRouter().GetRouter(hostname)
	if r == nil {
		return nil
	}
	return r.(*pathroute.PathRouter)
}

func (a *Adapter) GetHttpLocation(hostname string, locationId string) *httploc.HttpLocation {
	router := a.GetPathRouter(hostname)
	if router == nil {
		return nil
	}
	ilo := router.GetLocationById(locationId)
	if ilo == nil {
		return nil
	}
	return ilo.(*httploc.HttpLocation)
}

func (a *Adapter) GetHttpLocationLb(hostname string, locationId string) *roundrobin.RoundRobin {
	loc := a.GetHttpLocation(hostname, locationId)
	if loc == nil {
		return nil
	}
	return loc.GetLoadBalancer().(*roundrobin.RoundRobin)
}

func (a *Adapter) GetStats(hostname, locationId, endpointId string) *EndpointStats {
	rr := a.GetHttpLocationLb(hostname, locationId)
	if rr == nil {
		return nil
	}
	endpoint := rr.FindEndpointById(endpointId)
	if endpoint == nil {
		return nil
	}
	meterI := endpoint.GetMeter()
	if meterI == nil {
		return nil
	}
	meter := meterI.(*metrics.RollingMeter)

	return &EndpointStats{
		Successes:     meter.SuccessCount(),
		Failures:      meter.FailureCount(),
		PeriodSeconds: int(meter.WindowSize() / time.Second),
		FailRate:      meter.GetRate(),
	}
}
