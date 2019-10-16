package tracing

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"reflect"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	log "github.com/sirupsen/logrus"
)

type Middleware struct {
	handler http.Handler
}

func NewMiddleware(handler http.Handler) *Middleware {
	return &Middleware{
		handler: handler,
	}
}

func (c *Middleware) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	wireCtx, err := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		if err != opentracing.ErrSpanContextNotFound {
			log.Errorf("while extracting open tracing headers: %s", err)
		}
	}

	// Create the rootSpan using the wire context if available
	// If wireCtx == nil, a new root span will be created.
	serverSpan := opentracing.StartSpan(
		"vulcand",
		ext.RPCServerOption(wireCtx))

	// This spans all middleware configured for this proxy request
	// and is Finished() in the rtmcollect package just before the request
	// is passed off to oxy to be forwarded.
	span := serverSpan.Tracer().StartSpan("middleware",
		opentracing.ChildOf(serverSpan.Context()))

	// Construct a new context from the http.Request context with our span attached
	ctx := opentracing.ContextWithSpan(req.Context(), span)

	// Pass on the parent span via headers to the forwarded service
	err = serverSpan.Tracer().Inject(
		span.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(req.Header))
	if err != nil {
		log.Errorf("while injecting open tracing headers: %s", err)
	}

	wrapper := &ResponseWriterWrapper{writer: w}
	// Downstream middleware can retrieve the span using
	// opentracing.SpanFromContext(req.Context())
	c.handler.ServeHTTP(wrapper, req.WithContext(ctx))

	serverSpan.SetTag("http.status", wrapper.StatusCode())
	serverSpan.Finish()
}

type ResponseWriterWrapper struct {
	statusCode int
	writer     http.ResponseWriter
}

// CloseNotify this allows downstream connections to be terminated when the client terminates.
func (rw *ResponseWriterWrapper) CloseNotify() <-chan bool {
	if cn, ok := rw.writer.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	log.Warningf("Upstream ResponseWriter of type %v does not implement http.CloseNotifier. Returning dummy channel.", reflect.TypeOf(rw.writer))
	return make(<-chan bool)
}

// Hijack This allows connections to be hijacked for websockets for instance.
func (rw *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hi, ok := rw.writer.(http.Hijacker); ok {
		return hi.Hijack()
	}
	log.Warningf("Upstream ResponseWriter of type %v does not implement http.Hijacker. Returning dummy channel.", reflect.TypeOf(rw.writer))
	return nil, nil, fmt.Errorf("the wrapped response writer, does not implement http.Hijacker. It is of type: %v", reflect.TypeOf(rw.writer))
}

func (rw *ResponseWriterWrapper) Header() http.Header {
	return rw.writer.Header()
}

func (rw *ResponseWriterWrapper) Write(b []byte) (int, error) {
	return rw.writer.Write(b)
}

func (rw *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.writer.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapper) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}
