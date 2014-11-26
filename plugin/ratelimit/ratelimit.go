package ratelimit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/limit/tokenbucket"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/plugin"
)

// GetSpec is an entry point of a plugin and will be called to register this
// middleware plugin with vulcand.
func GetSpec() *plugin.MiddlewareSpec {
	cliFlags := []cli.Flag{
		cli.StringFlag{Name: "variable, var", Value: "client.ip", Usage: "variable to rate against, e.g. client.ip, request.host or request.header.X-Header"},
		cli.IntFlag{Name: "requests", Value: 1, Usage: "amount of requests"},
		cli.IntFlag{Name: "period", Value: 1, Usage: "rate limit period in seconds"},
		cli.IntFlag{Name: "burst", Value: 1, Usage: "allowed burst"},
		cli.StringFlag{Name: "configVar", Value: "", Usage: "variable to retrieve rate config from, e.g. request.header.X-Rates-Json"},
	}
	return &plugin.MiddlewareSpec{
		Type:      "ratelimit",
		FromOther: FromOther,
		FromCli:   FromCli,
		CliFlags:  cliFlags,
	}
}

// FromOther constructs middleware factory from `RateLimitSpec`.
func FromOther(config Config) (plugin.Middleware, error) {
	mapper, err := limit.VariableToMapper(config.Variable)
	if err != nil {
		return nil, err
	}

	configMapper, err := configMapperFromVar(config.ConfigVar)
	if err != nil {
		return nil, err
	}

	period := time.Duration(config.PeriodSeconds) * time.Second
	defaultRates := tokenbucket.NewRateSet()
	if err = defaultRates.Add(period, config.Requests, config.Burst); err != nil {
		return nil, err
	}

	return &Factory{
		config:       &config,
		mapper:       mapper,
		configMapper: configMapper,
		defaultRates: defaultRates,
	}, nil
}

// FromCli constructs the middleware from the command line arguments.
func FromCli(c *cli.Context) (plugin.Middleware, error) {
	return FromOther(
		Config{
			PeriodSeconds: int64(c.Int("period")),
			Requests:      int64(c.Int("requests")),
			Burst:         int64(c.Int("burst")),
			Variable:      c.String("var"),
			ConfigVar:     c.String("configVar")})
}

// Config defines configuration parameters of `Factory`.
type Config struct {
	// Period in seconds, e.g. 3600 to set up hourly rates.
	PeriodSeconds int64
	// Burst count, allows some extra variance for requests exceeding the
	// average rate.
	Burst int64
	// Variable defines how the limiting should be done. E.g. 'client.ip' or
	// 'request.header.X-My-Header'
	Variable string
	// Allowed average requests
	Requests int64
	// Variable that defines the source of token bucket configurations. This
	// parameter is optional. If omitted then the rate given via `Requests`,
	// `Burst`, and `PeriodSeconds` is used.
	ConfigVar string
}

// RateLimitFactory implements `middleware.Middleware`. Its `NewMiddleware`
// method creates a rate limiting middleware based on the token bucket
// algorithm http://en.wikipedia.org/wiki/Token_bucket.
type Factory struct {
	config       *Config
	mapper       limit.MapperFn
	configMapper tokenbucket.ConfigMapperFn
	defaultRates *tokenbucket.RateSet
	clock        timetools.TimeProvider
}

// Returns vulcan library compatible middleware
func (f *Factory) NewMiddleware() (middleware.Middleware, error) {
	return tokenbucket.NewLimiter(f.defaultRates, 0, f.mapper, f.configMapper, f.clock)
}

func (f *Factory) String() string {
	return fmt.Sprintf("reqs/%s=%d, burst=%d, var=%s, configVar=%s",
		time.Duration(f.config.PeriodSeconds)*time.Second, f.config.Requests,
		f.config.Burst, f.config.Variable, f.config.ConfigVar)
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
			period := time.Duration(s.PeriodSec) * time.Second
			if s.Burst == 0 {
				s.Burst = s.Average
			}
			if err = rateSet.Add(period, s.Average, s.Burst); err != nil {
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
	PeriodSec int64
	Average   int64
	Burst     int64
}
