package etcdng

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

func NewTLSConfig(opt Options) *tls.Config {
	var cfg *tls.Config = nil

	if opt.CertFile != "" && opt.KeyFile != "" {
		var rpool *x509.CertPool = nil
		if opt.CaFile != "" {
			if pemBytes, err := ioutil.ReadFile(opt.CaFile); err == nil {
				rpool = x509.NewCertPool()
				rpool.AppendCertsFromPEM(pemBytes)
			} else {
				log.WithError(err).Errorf("Error reading etcd cert CA File")
			}
		}

		if tlsCert, err := tls.LoadX509KeyPair(opt.CertFile, opt.KeyFile); err == nil {
			cfg = &tls.Config{
				RootCAs:            rpool,
				Certificates:       []tls.Certificate{tlsCert},
				InsecureSkipVerify: opt.InsecureSkipVerify,
			}
		} else {
			log.WithError(err).Errorf("Error loading KeyPair for TLS client")
		}
	}

	// If InsecureSkipVerify is provided, assume TLS
	if (opt.EnableTLS || opt.InsecureSkipVerify) && cfg == nil {
		cfg = &tls.Config{
			InsecureSkipVerify: opt.InsecureSkipVerify,
		}
	}
	return cfg
}
