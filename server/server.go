package server

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route"
	"github.com/mailgun/vulcand/backend"
	"time"
)

type Server interface {
	AddHostListener(host *backend.Host, router route.Router, l *backend.Listener) error

	HasHostListener(hostname, listenerId string) bool
	DeleteHostListener(hostname string, listenerId string) error
	DeleteHostListeners(hostname string) error

	UpdateHostCert(hostname string, cert *backend.Certificate) error
}

type Options struct {
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
}
