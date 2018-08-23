package etcdng

import "github.com/vulcand/vulcand/secret"

type Options struct {
	Consistency         string
	CaFile              string
	CertFile            string
	KeyFile             string
	SyncIntervalSeconds int64
	Username            string
	Password            string
	InsecureSkipVerify  bool
	EnableTLS           bool
	Debug               bool
	Box                 *secret.Box
}
