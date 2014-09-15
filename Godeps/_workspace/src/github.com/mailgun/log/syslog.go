package log

import (
	"log/syslog"
	"os"
	"path/filepath"
)

// Syslogger sends all your logs to syslog
// Note: the logs are going to MAIL_LOG facility
type sysLogger struct {
	writer *syslog.Writer
}

var newSyslogWriter = syslog.New // for mocking in tests

func NewSysLogger(config *LogConfig) (Logger, error) {
	writer, err := newSyslogWriter(syslog.LOG_MAIL, getAppName())
	if err != nil {
		return nil, err
	}

	return &sysLogger{writer: writer}, nil
}

// Get process name
func getAppName() string {
	return filepath.Base(os.Args[0])
}

func (l *sysLogger) Info(message string) {
	l.writer.Info(message)
}

func (l *sysLogger) Warning(message string) {
	l.writer.Warning(message)
}

func (l *sysLogger) Error(message string) {
	l.writer.Err(message)
}

func (l *sysLogger) Fatal(message string) {
	l.writer.Emerg(message)
}
