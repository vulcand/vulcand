package tracing

import (
	"io"

	"github.com/pkg/errors"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

func NewJaegerClient() (io.Closer, error) {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
		},
	}

	// TODO: Hook into logrus
	closer, err := cfg.InitGlobalTracer("vulcand", config.Logger(jaeger.StdLogger))
	if err != nil {
		return nil, errors.Wrap(err, "while initializing jaeger open tracing")
	}
	return closer, nil
}
