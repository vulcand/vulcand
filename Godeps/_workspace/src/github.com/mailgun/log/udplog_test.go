package log

import (
	"strings"

	. "gopkg.in/check.v1"
)

type UDPLoggerSuite struct {
}

var _ = Suite(&UDPLoggerSuite{})

func (s *UDPLoggerSuite) TestNewUDPLogger(c *C) {
	l, err := NewUDPLogger(Config{UDPLog, "info"})
	c.Assert(err, IsNil)
	c.Assert(l, NotNil)

	udplog := l.(*udpLogger)
	c.Assert(udplog.sev, Equals, SeverityInfo)
	c.Assert(udplog.w, NotNil)
}

func (s *UDPLoggerSuite) TestFormatMessage(c *C) {
	l, _ := NewUDPLogger(Config{UDPLog, "info"})

	udplog := l.(*udpLogger)

	message := udplog.FormatMessage(SeverityInfo, &CallerInfo{"filename", "filepath", "funcname", 42}, "hello %s", "world")
	c.Assert(strings.HasPrefix(message, DefaultCategory), Equals, true)
	c.Assert(strings.Contains(message, "filepath"), Equals, true)
	c.Assert(strings.Contains(message, "funcname"), Equals, true)
	c.Assert(strings.Contains(message, "42"), Equals, true)
	c.Assert(strings.Contains(message, "hello world"), Equals, true)
}
