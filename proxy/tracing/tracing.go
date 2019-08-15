package tracing

import (
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	log "github.com/sirupsen/logrus"
)

type Middleware struct {
	handler http.Handler
}

func NewMiddleware(handler http.Handler) *Middleware {
	return &Middleware{
		handler:   handler,
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
	defer serverSpan.Finish()

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

	// Downstream middleware can retrieve the span using
	// opentracing.SpanFromContext(req.Context())
	c.handler.ServeHTTP(w, req.WithContext(ctx))
}
