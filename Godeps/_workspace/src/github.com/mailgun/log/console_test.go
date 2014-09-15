package log

import (
	. "launchpad.net/gocheck"
)

type ConsoleLogSuite struct {
	logger Logger
}

var _ = Suite(&ConsoleLogSuite{})

func (s *ConsoleLogSuite) SetUpSuite(c *C) {
	config := &LogConfig{Name: "test"}
	s.logger, _ = NewConsoleLogger(config)
}

func (s *ConsoleLogSuite) TestNewConsoleLogger(c *C) {
	config := &LogConfig{Name: "testNew"}
	logger, err := NewConsoleLogger(config)
	c.Assert(logger, NotNil)
	c.Assert(err, IsNil)
}

func (s *ConsoleLogSuite) TestInfo(c *C) {
	s.logger.Info("test message")
}

func (s *ConsoleLogSuite) TestWarning(c *C) {
	s.logger.Warning("test message")
}

func (s *ConsoleLogSuite) TestError(c *C) {
	s.logger.Error("test message")
}

func (s *ConsoleLogSuite) TestFatal(c *C) {
	s.logger.Fatal("test message")
}
