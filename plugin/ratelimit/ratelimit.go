package ratelimit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit/tokenbucket"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/plugin"
)

// Spec is an entry point of a plugin and will be called to register this middleware plugin withing vulcand
func GetSpec() *plugin.MiddlewareSpec {
	cliFlags := []cli.Flag{
		cli.IntFlag{Name: "period", Value: 1, Usage: "rate limit period in seconds"},
		cli.IntFlag{Name: "requests", Value: 1, Usage: "amount of requests"},
		cli.IntFlag{Name: "burst", Value: 1, Usage: "allowed burst"},
		cli.StringFlag{Name: "variable, var", Value: "client.ip", Usage: "variable to rate against, e.g. client.ip, request.host or request.header.X-Header"},
		cli.StringFlag{Name: "rateVar", Value: "", Usage: "variable to retrieve rates from, e.g. request.header.X-Rates"},
	}
	return &plugin.MiddlewareSpec{
		Type:      "ratelimit",
		FromOther: FromOther,
		FromCli:   FromCli,
		CliFlags:  cliFlags,
	}
}

func FromOther(o RateLimit) (plugin.Middleware, error) {
	if o.Requests <= 0 {
		return nil, fmt.Errorf("requests should be > 0, got %d", o.Requests)
	}
	if o.Burst < 0 {
		return nil, fmt.Errorf("burst should be >= 0, got %d", o.Burst)
	}
	if o.PeriodSeconds <= 0 {
		return nil, fmt.Errorf("period seconds should be > 0, got %d", o.PeriodSeconds)
	}
	mapper, err := limit.VariableToMapper(o.Variable)
	if err != nil {
		return nil, err
	}
	configMapper, err := configMapperFromVar(o.RateVar)
	if err != nil {
		return nil, err
	}

	o.mapper = mapper
	o.configMapper = configMapper
	return &o, nil
}

// FromCli constructs a middleware instance from the command line parameters.
func FromCli(c *cli.Context) (plugin.Middleware, error) {
	return FromOther(
		RateLimit{
			PeriodSeconds: int64(c.Int("period")),
			Requests:      int64(c.Int("requests")),
			Burst:         int64(c.Int("burst")),
			Variable:      c.String("var"),
			RateVar:       c.String("rateVar")})
}

// Rate controls how many requests per period of time is allowed for a location.
// Existing implementation is based on the token bucket algorightm http://en.wikipedia.org/wiki/Token_bucket
type RateLimit struct {
	// Period in seconds, e.g. 3600 to set up hourly rates
	PeriodSeconds int64
	// Allowed average requests
	Requests int64
	// Burst count, allowes some extra variance for requests exceeding the average rate
	Burst int64
	// Variable defines how the limiting should be done. e.g. 'client.ip' or 'request.header.X-My-Header'
	Variable string
	// RateVar defines the source of rates configuration that should be used to
	// process a particular request. E.g. 'request.header.X-Rates'
	RateVar string

	mapper       limit.MapperFn
	configMapper tokenbucket.ConfigMapperFn
	clock        timetools.TimeProvider
}

// Returns vulcan library compatible middleware
func (r *RateLimit) NewMiddleware() (middleware.Middleware, error) {
	defaultRates := tokenbucket.NewRateSet()
	defaultRates.Add(time.Duration(r.PeriodSeconds)*time.Second, r.Requests, r.Burst)

	return tokenbucket.NewLimiter(defaultRates, 0, r.mapper, r.configMapper, r.clock)
}

func (rl *RateLimit) String() string {
	return fmt.Sprintf("reqs/%s=%d, burst=%d, var=%s, rateVar=%s",
		time.Duration(rl.PeriodSeconds)*time.Second, rl.Requests, rl.Burst, rl.Variable, rl.RateVar)
}

func configMapperFromVar(variable string) (tokenbucket.ConfigMapperFn, error) {
	if variable == "" {
		return nil, nil
	}

	m, err := limit.MakeTokenMapperFromVariable(variable)
	if err != nil {
		return nil, err
	}

	return func(r request.Request) (*tokenbucket.RateSet, error) {
		jsonString, err := m(r)
		if err != nil {
			return nil, err
		}

		var specs []rateSpec
		if err = json.Unmarshal([]byte(jsonString), &specs); err != nil {
			return nil, err
		}

		rateSet := tokenbucket.NewRateSet()
		for _, s := range specs {
			period := time.Duration(s.PeriodSeconds) * time.Second
			if s.Burst == 0 {
				s.Burst = s.Requests
			}
			if err = rateSet.Add(period, s.Requests, s.Burst); err != nil {
				return nil, err
			}
		}
		return rateSet, nil
	}, nil
}

// rateSpec is used to serialize token bucket rates to JSON. Note that the
// `burst` parameter can be omitted in the serialized form, in that case it is
// considered to be equal to `average`.
type rateSpec struct {
	PeriodSeconds int64
	Requests      int64
	Burst         int64
}
