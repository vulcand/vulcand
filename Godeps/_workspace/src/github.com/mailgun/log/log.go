package log

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
)

var loggers []Logger

var pid = os.Getpid()
var currentSeverity Severity

// Severity implementation is borrowed from glog, uses sync/atomic int32
type Severity int32

const (
	SeverityInfo Severity = iota
	SeverityWarn
	SeverityError
	SeverityFatal
)

var severityName = map[Severity]string{
	SeverityInfo:  "INFO",
	SeverityWarn:  "WARN",
	SeverityError: "ERROR",
	SeverityFatal: "FATAL",
}

// get returns the value of the severity.
func (s *Severity) Get() Severity {
	return Severity(atomic.LoadInt32((*int32)(s)))
}

// set sets the value of the severity.
func (s *Severity) Set(val Severity) {
	atomic.StoreInt32((*int32)(s), int32(val))
}

// less returns if this severity is greater than passed severity
func (s *Severity) Gt(val Severity) bool {
	return s.Get() > val
}

func (s Severity) String() string {
	n, ok := severityName[s]
	if !ok {
		return "UNKNOWN SEVERITY"
	}
	return n
}

func SeverityFromString(s string) (Severity, error) {
	s = strings.ToUpper(s)
	for k, val := range severityName {
		if val == s {
			return k, nil
		}
	}
	return -1, fmt.Errorf("unsupported severity: %s", s)
}

// Logger is a unified interface for all loggers.
type Logger interface {
	Info(string)
	Warning(string)
	Error(string)
	Fatal(string)
}

type logger struct{}

// Logging configuration to be passed to all loggers during initialization.
type LogConfig struct {
	Name string
}

// SetSeverity sets current logging severity. Acceptable values are SeverityInfo, SeverityWarn, SeverityError, SeverityFatal
func SetSeverity(s Severity) {
	currentSeverity.Set(s)
}

// GetSeverity returns currently set severity.
func GetSeverity() Severity {
	return currentSeverity
}

// Loggin initialization, must be called at the beginning of your cool app.
func Init(logConfigs []*LogConfig) error {
	for _, config := range logConfigs {
		logger, err := newLogger(config)
		if err != nil {
			return err
		}
		loggers = append(loggers, logger)
	}
	return nil
}

// Make a proper logger from a given configuration.
func newLogger(config *LogConfig) (Logger, error) {
	switch config.Name {
	case "console":
		return NewConsoleLogger(config)
	case "syslog":
		return NewSysLogger(config)
	}
	return nil, errors.New(fmt.Sprintf("Unknown logger: %v", config))
}

// Infof logs to the INFO log.
func Infof(format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityInfo) {
		return
	}
	message := makeMessage(currentSeverity, format, args...)
	for _, logger := range loggers {
		logger.Info(message)
	}
}

// Warningf logs to the WARNING and INFO logs.
func Warningf(format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityWarn) {
		return
	}
	message := makeMessage(currentSeverity, format, args...)
	for _, logger := range loggers {
		logger.Warning(message)
	}
}

// Errorf logs to the ERROR, WARNING, and INFO logs.
func Errorf(format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityError) {
		return
	}
	message := makeMessage(currentSeverity, format, args...)
	for _, logger := range loggers {
		logger.Error(message)
	}
}

// Fatalf logs to the FATAL, ERROR, WARNING, and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
func Fatalf(format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityFatal) {
		return
	}
	message := makeMessage(currentSeverity, format, args...)
	stacks := stackTraces()
	for _, logger := range loggers {
		logger.Fatal(message)
		logger.Fatal(stacks)
	}

	exit()
}

func makeMessage(sev Severity, format string, args ...interface{}) string {
	file, line := callerInfo()
	return fmt.Sprintf("PID:%d [%s:%d] %s", pid, file, line, fmt.Sprintf(format, args...))
}

// Return stack traces of all the running goroutines.
func stackTraces() string {
	trace := make([]byte, 100000)
	nbytes := runtime.Stack(trace, true)
	return string(trace[:nbytes])
}

// Return a file name and a line number.
func callerInfo() (string, int) {
	_, file, line, ok := runtimeCaller(3) // number of frames to the user's call.

	if !ok {
		file = "unknown"
		line = 0
	} else {
		slashPosition := strings.LastIndex(file, "/")
		if slashPosition >= 0 {
			file = file[slashPosition+1:]
		}
	}

	return file, line
}

// runtime functions for mocking

var runtimeCaller = runtime.Caller

var exit = func() {
	os.Exit(255)
}
