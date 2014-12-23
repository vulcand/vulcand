package server

import (
	"net/http"

	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location/httploc"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcand/backend"
)

type upstream struct {
	m    *MuxServer
	t    *http.Transport
	locs map[backend.LocationKey]*location
	up   backend.Upstream
}

func (u *upstream) addLocation(key backend.LocationKey, l *location) {
	u.locs[key] = l
}

func (u *upstream) deleteLocation(key backend.LocationKey) {
	delete(u.locs, key)
}

func (u *upstream) Close() error {
	u.t.CloseIdleConnections()
	return nil
}

func (u *upstream) update(up *backend.Upstream) error {
	if err := u.updateOptions(up.Options); err != nil {
		return err
	}
	return u.updateEndpoints(up.Endpoints)
}

func (u *upstream) updateOptions(opts backend.UpstreamOptions) error {
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

func (u *upstream) updateEndpoints(e []*backend.Endpoint) error {
	u.up.Endpoints = e
	for _, l := range u.locs {
		l.up = u
		if err := l.updateUpstream(l.up); err != nil {
			log.Errorf("failed to update %v err: %s", l, err)
		}
	}
	return nil
}

func newUpstream(m *MuxServer, up *backend.Upstream) (*upstream, error) {
	o, err := m.getTransportOptions(up)
	if err != nil {
		return nil, err
	}
	t := httploc.NewTransport(*o)
	return &upstream{
		m:    m,
		up:   *up,
		t:    t,
		locs: make(map[backend.LocationKey]*location),
	}, nil
}
