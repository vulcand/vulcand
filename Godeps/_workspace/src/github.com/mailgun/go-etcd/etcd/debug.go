package etcd

import (
	"fmt"
	"io/ioutil"
	"log"
)

var logger *etcdLogger

func SetLogger(l *log.Logger) {
	logger = &etcdLogger{l}
}

func GetLogger() *log.Logger {
	return logger.log
}

type etcdLogger struct {
	log *log.Logger
}

func (p *etcdLogger) Debug(args ...interface{}) {
	p.log.Println(fmt.Sprintf("DEBUG: %s", args))
}

func (p *etcdLogger) Debugf(f string, args ...interface{}) {
	p.log.Printf(fmt.Sprintf("DEBUG: %s", fmt.Sprintf(f, args)))
}

func (p *etcdLogger) Warning(args ...interface{}) {
	args[0] = "WARNING: " + args[0].(string)
	p.log.Println(fmt.Sprintf("WARNING: %s", args))
}

func (p *etcdLogger) Warningf(f string, args ...interface{}) {
	p.log.Printf(fmt.Sprintf("DEBUG: %s", fmt.Sprintf(f, args)))
}

func init() {
	// Default logger uses the go default log.
	SetLogger(log.New(ioutil.Discard, "go-etcd", log.LstdFlags))
}
