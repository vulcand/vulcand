package log

import (
	"log/syslog"
	"os"
	"path/filepath"
)

// Syslogger sends all your logs to syslog
// Note: the logs are going to MAIL_LOG facility
type sysLogger struct {
	info *syslog.Writer
	warn *syslog.Writer
	err  *syslog.Writer
}

var newSyslogWriter = syslog.New // for mocking in tests

func NewSysLogger(config *LogConfig) (Logger, error) {
	info, err := newSyslogWriter(syslog.LOG_MAIL|syslog.LOG_INFO, getAppName())
	if err != nil {
		return nil, err
	}

	warn, err := newSyslogWriter(syslog.LOG_MAIL|syslog.LOG_WARNING, getAppName())
	if err != nil {
		return nil, err
	}

	error, err := newSyslogWriter(syslog.LOG_MAIL|syslog.LOG_ERR, getAppName())
	if err != nil {
		return nil, err
	}

	return &sysLogger{
		info: info,
		warn: warn,
		err:  error,
	}, nil
}

// Get process name
func getAppName() string {
	return filepath.Base(os.Args[0])
}

func (l *sysLogger) infof(depth int, format string, args ...interface{}) {
	infof(depth, l.info, format, args...)
}

func (l *sysLogger) warningf(depth int, format string, args ...interface{}) {
	warningf(depth, l.warn, format, args...)
}

func (l *sysLogger) errorf(depth int, format string, args ...interface{}) {
	errorf(depth, l.err, format, args...)
}

func (l *sysLogger) fatalf(depth int, format string, args ...interface{}) {
	fatalf(depth, l.err, format, args...)
}

func (l *sysLogger) Infof(format string, args ...interface{}) {
	l.infof(1, format, args...)
}

func (l *sysLogger) Warningf(format string, args ...interface{}) {
	l.warningf(1, format, args...)
}

func (l *sysLogger) Errorf(format string, args ...interface{}) {
	l.errorf(1, format, args...)
}

func (l *sysLogger) Fatalf(format string, args ...interface{}) {
	l.fatalf(1, format, args...)
}
