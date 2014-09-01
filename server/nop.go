package server

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route"
	"github.com/mailgun/vulcand/backend"
)

type NopServer struct {
}

func (m *NopServer) AddHostListener(hostname string, router route.Router, l *backend.Listener) error {
	return nil
}

func (m *NopServer) HasHostListener(hostname, listenerId string) bool {
	return false
}

func (m *NopServer) DeleteHostListener(hostname string, listenerId string) error {
	return nil
}

func (m *NopServer) DeleteHostListeners(hostname string) error {
	return nil
}

func (m *NopServer) UpdateHostCert(hostname string, cert *backend.Certificate) error {
	return nil
}
