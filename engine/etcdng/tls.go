package etcdng

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

func NewTLSConfig(opt Options) *tls.Config {
	if opt.CertFile == "" && opt.KeyFile == "" && opt.CaFile == "" && !opt.EnableTLS && !opt.InsecureSkipVerify {
		return nil
	}

	cfg := tls.Config{
		InsecureSkipVerify: opt.InsecureSkipVerify,
	}

	if opt.CaFile != "" {
		if pemBytes, err := ioutil.ReadFile(opt.CaFile); err == nil {
			cfg.RootCAs = x509.NewCertPool()
			cfg.RootCAs.AppendCertsFromPEM(pemBytes)
		} else {
			log.WithError(err).Errorf("Error reading etcd cert CA File")
		}
	}

	if opt.CertFile != "" && opt.KeyFile != "" {
		if tlsCert, err := tls.LoadX509KeyPair(opt.CertFile, opt.KeyFile); err == nil {
			cfg.Certificates = []tls.Certificate{tlsCert}
		} else {
			log.WithError(err).Errorf("Error loading KeyPair for TLS client")
		}
	}

	return &cfg
}
