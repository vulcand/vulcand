package circuitbreaker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/mailgun/timetools"
	"github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcan/metrics"
	"github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcan/testutils"
	"github.com/mailgun/vulcan/threshold"
	. "gopkg.in/check.v1"
)

func TestCircuitBreaker(t *testing.T) { TestingT(t) }

type CBSuite struct {
	tm *timetools.FreezedTime
}

var _ = Suite(&CBSuite{
	tm: &timetools.FreezedTime{
		CurrentTime: time.Date(2012, 3, 4, 5, 6, 7, 0, time.UTC),
	},
})

var triggerNetRatio threshold.Predicate
var fallbackResponse middleware.Middleware
var fallbackRedirect middleware.Middleware

func (s CBSuite) SetUpSuite(c *C) {
	triggerNetRatio = MustParseExpression(`NetworkErrorRatio() > 0.5`)

	f, err := NewResponseFallback(Response{StatusCode: 400, Body: []byte("Come back later")})
	c.Assert(err, IsNil)
	fallbackResponse = f

	r, err := NewRedirectFallback(Redirect{URL: "http://localhost:5000"})
	c.Assert(err, IsNil)
	fallbackRedirect = r
}

func (s *CBSuite) advanceTime(d time.Duration) {
	s.tm.CurrentTime = s.tm.CurrentTime.Add(d)
}

func (s *CBSuite) new(c *C, condition threshold.Predicate, fallback middleware.Middleware, o Options) *CircuitBreaker {
	o.TimeProvider = s.tm
	cb, err := New(condition, fallback, o)
	c.Assert(err, IsNil)
	return cb
}

func (s *CBSuite) TestStandbyCycle(c *C) {
	cb := s.new(c, triggerNetRatio, fallbackResponse, Options{})

	// Nothing happened
	req := makeRequest(O{})
	re, err := cb.ProcessRequest(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	cb.ProcessResponse(req, req.Attempts[0])
	c.Assert(cb.state, Equals, cbState(stateStandby))

	re, err = cb.ProcessRequest(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)
}

func (s *CBSuite) TestFullCycle(c *C) {
	cb := s.new(c, triggerNetRatio, fallbackResponse,
		Options{
			FallbackDuration: 10 * time.Second,
			RecoveryDuration: 10 * time.Second,
		})

	req := makeRequest(O{})
	re, err := cb.ProcessRequest(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	cb.metrics = statsNetErrors(0.6)
	cb.ProcessResponse(req, req.Attempts[0])
	c.Assert(cb.state, Equals, cbState(stateTripped))

	re, err = cb.ProcessRequest(req)
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusBadRequest)

	// Some time has passed, but we are still in triggered state.
	s.advanceTime(9 * time.Second)
	re, err = cb.ProcessRequest(req)
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusBadRequest)
	c.Assert(cb.state, Equals, cbState(stateTripped))

	// We should be in recovering state by now
	okReq := makeRequest(O{})
	s.advanceTime(time.Second*1 + time.Millisecond)
	re, err = cb.ProcessRequest(okReq)
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusBadRequest)
	c.Assert(cb.state, Equals, cbState(stateRecovering))

	cb.ProcessResponse(okReq, okReq.Attempts[0])

	// 5 seconds after we should be allowing some requests to pass
	s.advanceTime(5 * time.Second)
	allowed := 0
	for i := 0; i < 100; i++ {
		re, err = cb.ProcessRequest(okReq)
		if re == nil && err == nil {
			allowed++
		}
	}
	c.Assert(allowed, Not(Equals), 0)

	// After some time, all is good and we should be in stand by mode again
	s.advanceTime(5*time.Second + time.Millisecond)
	re, err = cb.ProcessRequest(okReq)
	c.Assert(cb.state, Equals, cbState(stateStandby))
	c.Assert(err, IsNil)
	c.Assert(re, IsNil)
}

func (s *CBSuite) TestRedirect(c *C) {
	cb := s.new(c,
		triggerNetRatio,
		fallbackRedirect,
		Options{
			FallbackDuration: 10 * time.Second,
			RecoveryDuration: 10 * time.Second,
		})

	req := makeRequest(O{})
	cb.metrics = statsNetErrors(0.6)
	re, err := cb.ProcessRequest(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	cb.ProcessResponse(req, req.Attempts[0])
	c.Assert(cb.state, Equals, cbState(stateTripped))

	re, err = cb.ProcessRequest(req)
	c.Assert(re, IsNil)
	c.Assert(err, DeepEquals, &errors.RedirectError{URL: netutils.MustParseUrl("http://localhost:5000")})
}

func (s *CBSuite) TestTriggerDuringRecovery(c *C) {
	cb := s.new(c,
		triggerNetRatio,
		fallbackResponse,
		Options{
			FallbackDuration: 10 * time.Second,
			RecoveryDuration: 10 * time.Second,
			CheckPeriod:      time.Microsecond,
		})

	req := makeRequest(O{id: 8, stats: statsNetErrors(0.6)})
	re, err := cb.ProcessRequest(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	cb.metrics = statsNetErrors(0.6)

	cb.ProcessResponse(req, req.Attempts[0])
	c.Assert(cb.state, Equals, cbState(stateTripped))

	re, err = cb.ProcessRequest(req)
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusBadRequest)

	// We should be in recovering state by now
	okReq := makeRequest(O{stats: statsOK()})
	s.advanceTime(10*time.Second + time.Millisecond)
	re, err = cb.ProcessRequest(okReq)
	c.Assert(re, NotNil)
	c.Assert(err, IsNil)
	c.Assert(re.StatusCode, Equals, http.StatusBadRequest)
	c.Assert(cb.state, Equals, cbState(stateRecovering))
	cb.ProcessResponse(okReq, okReq.Attempts[0])

	// We have triggered it during recovery state and are going back to triggered state
	s.advanceTime(time.Millisecond)
	badReq := makeRequest(O{})
	cb.metrics = statsNetErrors(0.6)
	cb.ProcessRequest(badReq)
	cb.ProcessResponse(badReq, badReq.Attempts[0])
	c.Assert(cb.state, Equals, cbState(stateTripped))
	c.Assert(cb.until, Equals, s.tm.UtcNow().Add(10*time.Second))
}

func (s *CBSuite) TestSideEffects(c *C) {
	srv1Chan := make(chan *http.Request, 1)
	var srv1Body []byte
	srv1 := testutils.NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		c.Assert(err, IsNil)
		srv1Body = b
		w.Write([]byte("srv1"))
		srv1Chan <- r
	})
	defer srv1.Close()

	srv2Chan := make(chan *http.Request, 1)
	srv2 := testutils.NewTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("srv2"))
		r.ParseForm()
		srv2Chan <- r
	})
	defer srv2.Close()

	onTripped, err := NewWebhookSideEffect(
		Webhook{
			URL:     fmt.Sprintf("%s/post.json", srv1.URL),
			Method:  "POST",
			Headers: map[string][]string{"Content-Type": []string{"application/json"}},
			Body:    []byte(`{"Key": ["val1", "val2"]}`),
		})
	c.Assert(err, IsNil)

	onStandby, err := NewWebhookSideEffect(
		Webhook{
			URL:    fmt.Sprintf("%s/post", srv2.URL),
			Method: "POST",
			Form:   map[string][]string{"key": []string{"val1", "val2"}},
		})
	c.Assert(err, IsNil)

	cb := s.new(c,
		triggerNetRatio,
		fallbackResponse,
		Options{
			FallbackDuration: 10 * time.Second,
			RecoveryDuration: 10 * time.Second,
			OnTripped:        onTripped,
			OnStandby:        onStandby,
		})

	req := makeRequest(O{id: 8})
	cb.metrics = statsNetErrors(0.6)
	re, err := cb.ProcessRequest(req)
	c.Assert(re, IsNil)
	c.Assert(err, IsNil)

	cb.ProcessResponse(req, req.Attempts[0])
	c.Assert(cb.state, Equals, cbState(stateTripped))

	select {
	case req := <-srv1Chan:
		c.Assert(req.Method, Equals, "POST")
		c.Assert(req.URL.Path, Equals, "/post.json")
		c.Assert(string(srv1Body), Equals, `{"Key": ["val1", "val2"]}`)
		c.Assert(req.Header.Get("Content-Type"), Equals, "application/json")
	case <-time.After(time.Second):
		c.Error("timeout waiting for side effect to kick off")
	}

	// Transition to recovering state
	okReq := makeRequest(O{stats: statsOK()})
	s.advanceTime(10*time.Second + time.Millisecond)
	cb.ProcessRequest(okReq)
	c.Assert(cb.state, Equals, cbState(stateRecovering))
	cb.ProcessResponse(okReq, okReq.Attempts[0])

	// Going back to standby
	s.advanceTime(10*time.Second + time.Millisecond)
	cb.ProcessRequest(okReq)
	cb.ProcessResponse(okReq, req.Attempts[0])
	c.Assert(cb.state, Equals, cbState(stateStandby))

	select {
	case req := <-srv2Chan:
		c.Assert(req.Method, Equals, "POST")
		c.Assert(req.URL.Path, Equals, "/post")
		c.Assert(req.Form, DeepEquals, url.Values{"key": []string{"val1", "val2"}})
	case <-time.After(time.Second):
		c.Error("timeout waiting for side effect to kick off")
	}
}

func (s *CBSuite) TestInvalidParams(c *C) {
	cond, err := ParseExpression("NetworkErrorRatio() < 0.5")
	c.Assert(err, IsNil)

	r, err := NewResponseFallback(Response{StatusCode: 200, ContentType: "application/json", Body: []byte("yo")})
	c.Assert(err, IsNil)

	params := []struct {
		Condition threshold.Predicate
		Fallback  middleware.Middleware
		Options   Options
	}{
		{
			Condition: cond,
			Fallback:  nil,
			Options:   Options{},
		},
		{
			Condition: nil,
			Fallback:  r,
			Options:   Options{},
		},
		{
			Condition: cond,
			Fallback:  r,
			Options: Options{
				FallbackDuration: -1 * time.Millisecond,
			},
		},
	}
	for _, p := range params {
		cb, err := New(p.Condition, p.Fallback, p.Options)
		c.Assert(err, NotNil)
		c.Assert(cb, IsNil)
	}
}

func statsOK() *metrics.RoundTripMetrics {
	m, err := metrics.NewRoundTripMetrics(metrics.RoundTripOptions{})
	if err != nil {
		panic(err)
	}
	return m
}

func statsNetErrors(threshold float64) *metrics.RoundTripMetrics {
	m, err := metrics.NewRoundTripMetrics(metrics.RoundTripOptions{})
	if err != nil {
		panic(err)
	}
	for i := 0; i < 100; i++ {
		if i < int(threshold*100) {
			m.RecordMetrics(&request.BaseAttempt{Error: fmt.Errorf("boo")})
		} else {
			m.RecordMetrics(&request.BaseAttempt{Response: &http.Response{StatusCode: 200}})
		}
	}
	return m
}

func statsLatencyAtQuantile(quantile float64, value time.Duration) *metrics.RoundTripMetrics {
	m, err := metrics.NewRoundTripMetrics(metrics.RoundTripOptions{})
	if err != nil {
		panic(err)
	}
	m.RecordMetrics(&request.BaseAttempt{Duration: value})
	return m
}

func statsResponseCodes(codes ...statusCode) *metrics.RoundTripMetrics {
	m, err := metrics.NewRoundTripMetrics(metrics.RoundTripOptions{})
	if err != nil {
		panic(err)
	}
	for _, c := range codes {
		for i := int64(0); i < c.Count; i++ {
			m.RecordMetrics(&request.BaseAttempt{Response: &http.Response{StatusCode: c.Code}})
		}
	}

	return m
}

type statusCode struct {
	Code  int
	Count int64
}
type O struct {
	stats      *metrics.RoundTripMetrics
	id         int64
	noAttempts bool
}

func makeRequest(o O) *request.BaseRequest {
	req := request.NewBaseRequest(&http.Request{URL: &url.URL{}}, o.id, nil)
	if o.noAttempts {
		return req
	}
	req.SetUserData(cbreakerMetrics, o.stats)
	req.Attempts = []request.Attempt{
		&request.BaseAttempt{},
	}
	return req
}

func expr(v string) threshold.Predicate {
	e, err := ParseExpression(v)
	if err != nil {
		panic(err)
	}
	return e
}
