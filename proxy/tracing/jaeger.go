package tracing

import (
	"io"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

type logWrapper struct {
	logger *log.Logger
}

// Error logs a message at error priority
func (lw *logWrapper) Error(msg string) {
	lw.logger.Error(msg)
}

// Infof logs a message at info priority
func (lw *logWrapper) Infof(msg string, args ...interface{}) {
	lw.logger.Infof(msg, args...)
}

func NewJaegerClient(debug bool) (io.Closer, error) {
	cfg := &config.Configuration{}

	if debug {
		cfg = &config.Configuration{
			Sampler: &config.SamplerConfig{
				Type:  jaeger.SamplerTypeConst,
				Param: 1,
			},
			Reporter: &config.ReporterConfig{
				LogSpans: true,
			},
		}
	}

	closer, err := cfg.InitGlobalTracer("vulcand",
		config.Logger(&logWrapper{logger: log.StandardLogger()}))
	if err != nil {
		return nil, errors.Wrap(err, "while initializing jaeger open tracing")
	}
	return closer, nil
}
