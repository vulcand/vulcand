package service

import (
	"flag"
	"fmt"
	"github.com/mailgun/go-etcd/etcd"
)

type Options struct {
	ApiPort         int
	ApiInterface    string
	PidPath         string
	Port            int
	Interface       string
	CertPath        string
	EtcdNodes       listOptions
	EtcdKey         string
	EtcdConsistency string
	Log             string
}

// Helper to parse options that can occur several times, e.g. cassandra nodes
type listOptions []string

func (o *listOptions) String() string {
	return fmt.Sprint(*o)
}

func (o *listOptions) Set(value string) error {
	*o = append(*o, value)
	return nil
}

func ParseCommandLine() (options Options, err error) {
	flag.Var(&options.EtcdNodes, "etcd", "Etcd discovery service API endpoints")
	flag.StringVar(&options.EtcdKey, "etcdKey", "vulcand", "Etcd key for storing configuration")
	flag.StringVar(&options.EtcdConsistency, "etcdConsistency", etcd.STRONG_CONSISTENCY, "Etcd consistency")
	flag.StringVar(&options.PidPath, "pidPath", "", "Path to write PID file to")
	flag.IntVar(&options.Port, "port", 8181, "Port to listen on")
	flag.IntVar(&options.ApiPort, "apiPort", 8182, "Port to provide api on")
	flag.StringVar(&options.Interface, "interface", "", "Interface to bind to")
	flag.StringVar(&options.ApiInterface, "apiInterface", "", "Interface to for API to bind to")
	flag.StringVar(&options.CertPath, "certPath", "", "Certificate to use (enables TLS)")
	flag.StringVar(&options.Log, "log", "console", "Logging to use (syslog or console)")
	flag.Parse()
	return options, nil
}
