package logrus_logstash

import (
	"net"

	"github.com/Sirupsen/logrus"
)

// Hook represents a connection to a Logstash instance
type Hook struct {
	conn    net.Conn
	appName string
}

// NewHook creates a new hook to a Logstash instance, which listens on
// `protocol`://`address`.
func NewHook(protocol, address, appName string) (*Hook, error) {
	conn, err := net.Dial(protocol, address)
	if err != nil {
		return nil, err
	}
	return &Hook{conn: conn, appName: appName}, nil
}

func (h *Hook) Fire(entry *logrus.Entry) error {
	formatter := LogstashFormatter{Type: h.appName}
	dataBytes, err := formatter.Format(entry)
	if err != nil {
		return err
	}
	if _, err = h.conn.Write(dataBytes); err != nil {
		return err
	}
	return nil
}

func (h *Hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
