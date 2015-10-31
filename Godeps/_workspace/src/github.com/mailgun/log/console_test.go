package log

import (
	"bytes"
	"os"

	. "gopkg.in/check.v1"
)

type WriterLoggerSuite struct {
}

var _ = Suite(&WriterLoggerSuite{})

func (s *WriterLoggerSuite) TestWriter(c *C) {
	// INFO logger should log INFO, WARN and ERROR
	l := &writerLogger{SeverityInfo, &bytes.Buffer{}}
	c.Assert(l.Writer(SeverityInfo), NotNil)
	c.Assert(l.Writer(SeverityWarning), NotNil)
	c.Assert(l.Writer(SeverityError), NotNil)

	// WARN logger should log WARN and ERROR
	l = &writerLogger{SeverityWarning, &bytes.Buffer{}}
	c.Assert(l.Writer(SeverityInfo), IsNil)
	c.Assert(l.Writer(SeverityWarning), NotNil)
	c.Assert(l.Writer(SeverityError), NotNil)

	// ERROR logger should log only ERROR
	l = &writerLogger{SeverityError, &bytes.Buffer{}}
	c.Assert(l.Writer(SeverityInfo), IsNil)
	c.Assert(l.Writer(SeverityWarning), IsNil)
	c.Assert(l.Writer(SeverityError), NotNil)
}

type ConsoleLoggerSuite struct {
}

var _ = Suite(&ConsoleLoggerSuite{})

func (s *ConsoleLoggerSuite) TestNewConsoleLogger(c *C) {
	l, err := NewConsoleLogger(Config{Console, "info"})
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	console := l.(*consoleLogger)
	c.Assert(console.sev, Equals, SeverityInfo)
	c.Assert(console.w, Equals, os.Stdout)
}
