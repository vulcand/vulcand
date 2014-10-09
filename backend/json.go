package backend

import (
	"encoding/json"
	"fmt"
	"github.com/mailgun/vulcand/plugin"
)

type rawHosts struct {
	Hosts []json.RawMessage
}

type rawHost struct {
	Name      string
	Locations []json.RawMessage
}

type rawLocation struct {
	Hostname    string
	Path        string
	Id          string
	Upstream    *Upstream
	Middlewares []json.RawMessage
	Options     LocationOptions
	Stats       RoundTripStats
}

type rawMiddleware struct {
	Id         string
	Type       string
	Priority   int
	Middleware json.RawMessage
}

func HostsFromJSON(in []byte, getter plugin.SpecGetter) ([]*Host, error) {
	var hs *rawHosts
	err := json.Unmarshal(in, &hs)
	if err != nil {
		return nil, err
	}
	out := []*Host{}
	if hs.Hosts != nil {
		for _, raw := range hs.Hosts {
			h, err := HostFromJSON(raw, getter)
			if err != nil {
				return nil, err
			}
			out = append(out, h)
		}
	}
	return out, nil
}

func HostFromJSON(in []byte, getter plugin.SpecGetter) (*Host, error) {
	var h *rawHost
	err := json.Unmarshal(in, &h)
	if err != nil {
		return nil, err
	}
	out, err := NewHost(h.Name)
	if err != nil {
		return nil, err
	}
	if h.Locations != nil {
		for _, raw := range h.Locations {
			l, err := LocationFromJSON(raw, getter)
			if err != nil {
				return nil, err
			}
			out.Locations = append(out.Locations, l)
		}
	}
	return out, nil
}

func ListenerFromJSON(in []byte) (*Listener, error) {
	var l *Listener
	err := json.Unmarshal(in, &l)
	if err != nil {
		return nil, err
	}
	return NewListener(l.Id, l.Protocol, l.Address.Network, l.Address.Address)
}

func KeyPairFromJSON(in []byte) (*KeyPair, error) {
	var c *KeyPair
	err := json.Unmarshal(in, &c)
	if err != nil {
		return nil, err
	}
	return NewKeyPair(c.Cert, c.Key)
}

func LocationFromJSON(in []byte, getter plugin.SpecGetter) (*Location, error) {
	var l *rawLocation
	err := json.Unmarshal(in, &l)
	if err != nil {
		return nil, err
	}
	loc, err := NewLocationWithOptions(l.Hostname, l.Id, l.Path, l.Upstream.Id, l.Options)
	if err != nil {
		return nil, err
	}
	loc.Upstream = l.Upstream
	loc.Stats = l.Stats
	for _, el := range l.Middlewares {
		m, err := MiddlewareFromJSON(el, getter)
		if err != nil {
			return nil, err
		}
		loc.Middlewares = append(loc.Middlewares, m)
	}
	return loc, nil
}

func LocationOptionsFromJSON(in []byte) (*LocationOptions, error) {
	var o *LocationOptions
	err := json.Unmarshal(in, &o)
	if err != nil {
		return nil, err
	}
	if _, err := parseLocationOptions(*o); err != nil {
		return nil, err
	}
	return o, nil
}

func MiddlewareFromJSON(in []byte, getter plugin.SpecGetter) (*MiddlewareInstance, error) {
	var ms *rawMiddleware
	err := json.Unmarshal(in, &ms)
	if err != nil {
		return nil, err
	}
	spec := getter(ms.Type)
	if spec == nil {
		return nil, fmt.Errorf("middleware of type %s is not supported", ms.Type)
	}
	m, err := spec.FromJSON(ms.Middleware)
	if err != nil {
		return nil, err
	}
	return &MiddlewareInstance{
		Id:         ms.Id,
		Type:       ms.Type,
		Middleware: m,
		Priority:   ms.Priority,
	}, nil
}

func UpstreamFromJSON(in []byte) (*Upstream, error) {
	var u *Upstream
	err := json.Unmarshal(in, &u)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func EndpointFromJSON(in []byte) (*Endpoint, error) {
	var e *Endpoint
	err := json.Unmarshal(in, &e)
	if err != nil {
		return nil, err
	}
	return NewEndpoint(e.UpstreamId, e.Id, e.Url)
}
