package log

import (
	"fmt"
	"io"
	"os"
	"time"
)

// writerLogger outputs the logs to the underlying writer
type writerLogger struct {
	w io.Writer
}

func NewConsoleLogger(config *LogConfig) (Logger, error) {
	return &writerLogger{w: os.Stdout}, nil
}

func (l *writerLogger) infof(depth int, format string, args ...interface{}) {
	infof(depth, l.w, l.format(format), args...)
}

func (l *writerLogger) warningf(depth int, format string, args ...interface{}) {
	warningf(depth, l.w, l.format(format), args...)
}

func (l *writerLogger) errorf(depth int, format string, args ...interface{}) {
	errorf(depth, l.w, l.format(format), args...)
}

func (l *writerLogger) fatalf(depth int, format string, args ...interface{}) {
	fatalf(depth, l.w, l.format(format), args...)
}

func (l *writerLogger) Infof(format string, args ...interface{}) {
	l.infof(1, format, args...)
}

func (l *writerLogger) Warningf(format string, args ...interface{}) {
	l.warningf(1, format, args...)
}

func (l *writerLogger) Errorf(format string, args ...interface{}) {
	l.errorf(1, format, args...)
}

func (l *writerLogger) Fatalf(format string, args ...interface{}) {
	l.fatalf(1, format, args...)
}

func (l *writerLogger) format(format string) string {
	return fmt.Sprintf("%v: %v\n", time.Now().UTC().Format(time.StampMilli), format)
}
