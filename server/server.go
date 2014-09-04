package server

import (
	"time"

	"github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/connwatch"
)

type Server interface {
	UpsertHost(host *backend.Host) error
	DeleteHost(hostname string) error
	UpdateHostCert(hostname string, cert *backend.Certificate) error

	AddHostListener(host *backend.Host, l *backend.Listener) error
	DeleteHostListener(host *backend.Host, listenerId string) error

	UpsertLocation(host *backend.Host, loc *backend.Location) error
	DeleteLocation(host *backend.Host, locationId string) error

	UpdateLocationUpstream(host *backend.Host, loc *backend.Location) error
	UpdateLocationPath(host *backend.Host, loc *backend.Location, path string) error
	UpdateLocationOptions(host *backend.Host, loc *backend.Location) error

	UpsertLocationMiddleware(host *backend.Host, loc *backend.Location, mi *backend.MiddlewareInstance) error
	DeleteLocationMiddleware(host *backend.Host, loc *backend.Location, mType, mId string) error

	AddEndpoint(upstream *backend.Upstream, e *backend.Endpoint, affectedLocations []*backend.Location) error
	DeleteEndpoint(upstream *backend.Upstream, endpointId string, affectedLocations []*backend.Location) error

	HijackListeners(Server) error

	GetConnWatcher() *connwatch.ConnectionWatcher
	GetStats(hostname, locationId string, e *backend.Endpoint) *backend.EndpointStats

	Start() error
	Stop(wait bool)
}

type Options struct {
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxHeaderBytes  int
	DefaultListener *backend.Listener
}

type NewServerFn func(id int) (Server, error)
