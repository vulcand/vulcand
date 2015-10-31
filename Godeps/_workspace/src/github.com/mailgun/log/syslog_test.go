package log

import (
	"bytes"

	. "gopkg.in/check.v1"
)

type SysLoggerSuite struct {
}

var _ = Suite(&SysLoggerSuite{})

func (s *SysLoggerSuite) TestWriter(c *C) {
	debug, info, warning, error := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}

	// DEBUG logger should log DEBUG, INFO, WARN and ERROR
	l := &sysLogger{SeverityDebug, debug, info, warning, error}
	c.Assert(l.Writer(SeverityDebug), Equals, debug)
	c.Assert(l.Writer(SeverityInfo), Equals, info)
	c.Assert(l.Writer(SeverityWarning), Equals, warning)
	c.Assert(l.Writer(SeverityError), Equals, error)

	// INFO logger should log INFO, WARN and ERROR
	l = &sysLogger{SeverityInfo, debug, info, warning, error}
	c.Assert(l.Writer(SeverityDebug), IsNil)
	c.Assert(l.Writer(SeverityInfo), Equals, info)
	c.Assert(l.Writer(SeverityWarning), Equals, warning)
	c.Assert(l.Writer(SeverityError), Equals, error)

	// WARN logger should log WARN and ERROR
	l = &sysLogger{SeverityWarning, debug, info, warning, error}
	c.Assert(l.Writer(SeverityDebug), IsNil)
	c.Assert(l.Writer(SeverityInfo), IsNil)
	c.Assert(l.Writer(SeverityWarning), Equals, warning)
	c.Assert(l.Writer(SeverityError), Equals, error)

	// ERROR logger should log only ERROR
	l = &sysLogger{SeverityError, debug, info, warning, error}
	c.Assert(l.Writer(SeverityDebug), IsNil)
	c.Assert(l.Writer(SeverityInfo), IsNil)
	c.Assert(l.Writer(SeverityWarning), IsNil)
	c.Assert(l.Writer(SeverityError), Equals, error)
}

func (s *SysLoggerSuite) TestNewSysLogger(c *C) {
	l, err := NewSysLogger(Config{Syslog, "debug"})
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	syslog := l.(*sysLogger)
	c.Assert(syslog.sev, Equals, SeverityDebug)
	c.Assert(syslog.debugW, NotNil)
	c.Assert(syslog.infoW, NotNil)
	c.Assert(syslog.warnW, NotNil)
	c.Assert(syslog.errorW, NotNil)
}
