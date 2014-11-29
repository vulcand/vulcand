package log

// aggrLoger outputs the logs to the underlying writer
type aggrLogger struct {
	loggers []Logger
}

func (l *aggrLogger) add(lg Logger) {
	l.loggers = append(l.loggers, lg)
}

func (l *aggrLogger) infof(depth int, format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityInfo) {
		return
	}
	for _, logger := range l.loggers {
		logger.infof(depth+1, format, args...)
	}
}

func (l *aggrLogger) warningf(depth int, format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityWarn) {
		return
	}
	for _, logger := range l.loggers {
		logger.warningf(depth+1, format, args...)
	}
}

func (l *aggrLogger) errorf(depth int, format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityError) {
		return
	}
	for _, logger := range l.loggers {
		logger.errorf(depth+1, format, args...)
	}
}

func (l *aggrLogger) fatalf(depth int, format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityFatal) {
		return
	}
	for _, logger := range l.loggers {
		logger.fatalf(depth+1, format, args...)
	}
	exit()
}

func (l *aggrLogger) Infof(format string, args ...interface{}) {
	l.infof(1, format, args...)
}

func (l *aggrLogger) Warningf(format string, args ...interface{}) {
	l.warningf(1, format, args...)
}

func (l *aggrLogger) Errorf(format string, args ...interface{}) {
	l.errorf(1, format, args...)
}

func (l *aggrLogger) Fatalf(format string, args ...interface{}) {
	l.fatalf(1, format, args...)
}

var logger = &aggrLogger{loggers: []Logger{}}
