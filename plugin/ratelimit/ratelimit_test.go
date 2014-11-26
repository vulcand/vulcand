package ratelimit

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/timetools"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/plugin"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestRL(t *testing.T) { TestingT(t) }

type RateLimitSuite struct {
	clock *timetools.FreezedTime
}

func (s *RateLimitSuite) SetUpSuite(c *C) {
	s.clock = &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	}
}

var _ = Suite(&RateLimitSuite{})

// One of the most important tests:
// Make sure the RateLimit spec is compatible and will be accepted by middleware registry
func (s *RateLimitSuite) TestSpecIsOK(c *C) {
	c.Assert(plugin.NewRegistry().AddSpec(GetSpec()), IsNil)
}

func (s *RateLimitSuite) TestFromOther(c *C) {
	rlf, err := FromOther(
		Config{
			PeriodSeconds: 1,
			Requests:      1,
			Burst:         10,
			Variable:      "client.ip",
			ConfigVar:     "request.header.X-Rates",
		})
	c.Assert(rlf, NotNil)
	c.Assert(err, IsNil)
	c.Assert(fmt.Sprint(rlf), Equals, "reqs/1s=1, burst=10, var=client.ip, configVar=request.header.X-Rates")

	out, err := rlf.NewMiddleware()
	c.Assert(out, NotNil)
	c.Assert(err, IsNil)
}

func (s *RateLimitSuite) TestFromOtherNoConfigVar(c *C) {
	rlf, err := FromOther(
		Config{
			PeriodSeconds: 1,
			Requests:      1,
			Burst:         10,
			Variable:      "client.ip",
			ConfigVar:     "",
		})
	c.Assert(rlf, NotNil)
	c.Assert(err, IsNil)

	out, err := rlf.NewMiddleware()
	c.Assert(out, NotNil)
	c.Assert(err, IsNil)
}

func (s *RateLimitSuite) TestFromOtherBadParams(c *C) {
	// Unknown variable
	_, err := FromOther(
		Config{
			PeriodSeconds: 1,
			Requests:      1,
			Burst:         10,
			Variable:      "foo",
			ConfigVar:     "request.header.X-Rates-Json",
		})
	c.Assert(err, NotNil)

	// Negative requests
	_, err = FromOther(
		Config{
			PeriodSeconds: 1,
			Requests:      -1,
			Burst:         10,
			Variable:      "client.ip",
			ConfigVar:     "request.header.X-Rates-Json",
		})
	c.Assert(err, NotNil)

	// Negative burst
	_, err = FromOther(
		Config{
			PeriodSeconds: 1,
			Requests:      1,
			Burst:         -1,
			Variable:      "client.ip",
			ConfigVar:     "request.header.X-Rates-Json",
		})
	c.Assert(err, NotNil)

	// Negative period
	_, err = FromOther(
		Config{
			PeriodSeconds: -1,
			Requests:      1,
			Burst:         10,
			Variable:      "client.ip",
			ConfigVar:     "request.header.X-Rates-Json",
		})
	c.Assert(err, NotNil)

	// Unknown config variable
	_, err = FromOther(
		Config{
			PeriodSeconds: 1,
			Requests:      1,
			Burst:         10,
			Variable:      "client.ip",
			ConfigVar:     "foo",
		})
	c.Assert(err, NotNil)
}

func (s *RateLimitSuite) TestFromCli(c *C) {
	app := cli.NewApp()
	app.Name = "test"
	app.Flags = GetSpec().CliFlags
	executed := false
	app.Action = func(ctx *cli.Context) {
		executed = true
		out, err := FromCli(ctx)
		c.Assert(out, NotNil)
		c.Assert(err, IsNil)

		rlf := out.(*Factory)
		m, err := rlf.NewMiddleware()
		c.Assert(m, NotNil)
		c.Assert(err, IsNil)
	}
	app.Run([]string{"test", "--var=client.ip", "--requests=10", "--burst=3", "--period=4"})
	c.Assert(executed, Equals, true)
}

// Middleware instance created by the factory is using rates configuration
// from the respective request header.
func (s *RateLimitSuite) TestRequestProcessing(c *C) {
	// Given
	rlf, _ := FromOther(
		Config{
			PeriodSeconds: 1,
			Requests:      1,
			Burst:         1,
			Variable:      "client.ip",
			ConfigVar:     "request.header.X-Rates",
		})
	// Inject deterministic time provider into the middleware instance
	(rlf.(*Factory)).clock = s.clock

	rli, _ := rlf.NewMiddleware()

	request := &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: "1.2.3.4",
			Header: http.Header(map[string][]string{
				"X-Rates": []string{`[{"PeriodSec": 1, "Average": 2}]`}}),
		},
	}

	// When/Then: The configured rate is applied, which 2 request/second, note
	// that the default rate is 1 request/second.
	response, err := rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = rli.ProcessRequest(request) // Rejected
	c.Assert(response, NotNil)
	c.Assert(err, IsNil)

	s.clock.Sleep(time.Second)
	response, err = rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
}

func (s *RateLimitSuite) TestRequestProcessingEmptyConfig(c *C) {
	// Given
	rlf, _ := FromOther(
	Config{
		PeriodSeconds: 1,
		Requests:      1,
		Burst:         1,
		Variable:      "client.ip",
		ConfigVar:     "request.header.X-Rates",
	})
	// Inject deterministic time provider into the middleware instance
	(rlf.(*Factory)).clock = s.clock

	rli, _ := rlf.NewMiddleware()

	request := &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: "1.2.3.4",
			Header: http.Header(map[string][]string{
				"X-Rates": []string{`[]`}}),
		},
	}

	// When/Then: The default rate of 1 request/second is used.
	response, err := rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = rli.ProcessRequest(request) // Rejected
	c.Assert(response, NotNil)
	c.Assert(err, IsNil)

	s.clock.Sleep(time.Second)
	response, err = rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
}

func (s *RateLimitSuite) TestRequestProcessingNoHeader(c *C) {
	// Given
	rlf, _ := FromOther(
	Config{
		PeriodSeconds: 1,
		Requests:      1,
		Burst:         1,
		Variable:      "client.ip",
		ConfigVar:     "request.header.X-Rates",
	})
	// Inject deterministic time provider into the middleware instance
	(rlf.(*Factory)).clock = s.clock

	rli, _ := rlf.NewMiddleware()

	request := &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: "1.2.3.4",
		},
	}

	// When/Then: The default rate of 1 request/second is used.
	response, err := rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = rli.ProcessRequest(request) // Rejected
	c.Assert(response, NotNil)
	c.Assert(err, IsNil)

	s.clock.Sleep(time.Second)
	response, err = rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
}

// If the rate set from the HTTP header has more then one rate for the same
// time period defined, then the one mentioned in the list last is used.
func (s *RateLimitSuite) TestRequestInvalidConfig(c *C) {
	// Given
	rlf, _ := FromOther(
	Config{
		PeriodSeconds: 1,
		Requests:      1,
		Burst:         1,
		Variable:      "client.ip",
		ConfigVar:     "request.header.X-Rates",
	})
	// Inject deterministic time provider into the middleware instance
	(rlf.(*Factory)).clock = s.clock

	rli, _ := rlf.NewMiddleware()

	request := &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: "1.2.3.4",
			Header: http.Header(map[string][]string{
				"X-Rates": []string{`[{"PeriodSec": -1, "Average": 10}]`}}),
		},
	}

	// When/Then: The default rate of 1 request/second is used.
	response, err := rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = rli.ProcessRequest(request) // Rejected
	c.Assert(response, NotNil)
	c.Assert(err, IsNil)

	s.clock.Sleep(time.Second)
	response, err = rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
}

// If the rate set from the HTTP header has more then one rate for the same
// time period defined, then the one mentioned in the list last is used.
func (s *RateLimitSuite) TestRequestProcessingAmbiguousConfig(c *C) {
	// Given
	rlf, _ := FromOther(
	Config{
		PeriodSeconds: 1,
		Requests:      1,
		Burst:         1,
		Variable:      "client.ip",
		ConfigVar:     "request.header.X-Rates",
	})
	// Inject deterministic time provider into the middleware instance
	(rlf.(*Factory)).clock = s.clock

	rli, _ := rlf.NewMiddleware()

	request := &request.BaseRequest{
		HttpRequest: &http.Request{
			RemoteAddr: "1.2.3.4",
			Header: http.Header(map[string][]string{
				"X-Rates": []string{`[{"PeriodSec": 1, "Average": 10},
					                  {"PeriodSec": 1, "Average": 3}]`}}),
		},
	}

	// When/Then: The last of configured rates with the same period is applied,
    // which 3 request/second, note that the default rate is 1 request/second.
	response, err := rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
	response, err = rli.ProcessRequest(request) // Rejected
	c.Assert(response, NotNil)
	c.Assert(err, IsNil)

	s.clock.Sleep(time.Second)
	response, err = rli.ProcessRequest(request) // Processed
	c.Assert(response, IsNil)
	c.Assert(err, IsNil)
}
