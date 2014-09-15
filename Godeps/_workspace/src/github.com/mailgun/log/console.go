package log

import (
	"fmt"
	"time"
)

// Console logger is for dev mode, it prints the logs to the terminal.
// Note: don't use this logger in production.
type consoleLogger struct{}

func NewConsoleLogger(config *LogConfig) (Logger, error) {
	return &consoleLogger{}, nil
}

func (l *consoleLogger) Info(message string) {
	l.Print("INFO ", message)
}

func (l *consoleLogger) Warning(message string) {
	l.Print("WARN ", message)
}

func (l *consoleLogger) Error(message string) {
	l.Print("ERROR", message)
}

func (l *consoleLogger) Fatal(message string) {
	l.Print("FATAL", message)
}

func (l *consoleLogger) Print(severity string, message string) {
	fmt.Printf("%v %v: %v\n", severity, time.Now().UTC().Format(time.StampMilli), message)
}
