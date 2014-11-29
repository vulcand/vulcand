package server

import (
	"net/http"

	"github.com/mailgun/vulcand/engine"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"
)

type backend struct {
	m  *MuxServer
	t  *http.Transport
	fs map[engine.LocationKey]*frontend
	up backend.Upstream
}

func (u *backend) addLocation(key engine.LocationKey, l *location) {
	u.locs[key] = l
}

func (u *backend) deleteLocation(key engine.LocationKey) {
	delete(u.locs, key)
}

func (u *backend) Close() error {
	u.t.CloseIdleConnections()
	return nil
}

func (u *backend) update(up *engine.Backend) error {
	if err := u.updateOptions(up.Options); err != nil {
		return err
	}
	return u.updateEndpoints(up.Endpoints)
}

func (u *backend) updateOptions(opts engine.BackendOptions) error {
	// Nothing changed in transport options
	if u.up.Options.Equals(opts) {
		return nil
	}
	u.up.Options = opts

	o, err := u.m.getTransportOptions(&u.up)
	if err != nil {
		return err
	}
	t := httploc.NewTransport(*o)
	u.t.CloseIdleConnections()
	u.t = t
	for _, l := range u.locs {
		if err := l.hloc.SetTransport(u.t); err != nil {
			log.Errorf("Failed to set transport: %v", err)
		}
	}
	return nil
}

func (u *backend) updateEndpoints(e []*engine.Endpoint) error {
	u.up.Endpoints = e
	for _, l := range u.locs {
		l.up = u
		if err := l.updateBackend(l.up); err != nil {
			log.Errorf("failed to update %v err: %s", l, err)
		}
	}
	return nil
}

func newBackend(m *MuxServer, up *engine.Backend) (*backend, error) {
	o, err := m.getTransportOptions(up)
	if err != nil {
		return nil, err
	}
	t := httploc.NewTransport(*o)
	return &backend{
		m:    m,
		up:   *up,
		t:    t,
		locs: make(map[engine.LocationKey]*location),
	}, nil
}
