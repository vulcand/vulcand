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
}

type rawMiddleware struct {
	Id         string
	Type       string
	Priority   int
	Middleware json.RawMessage
}

func HostsFromJson(in []byte, getter plugin.SpecGetter) ([]*Host, error) {
	var hs *rawHosts
	err := json.Unmarshal(in, &hs)
	if err != nil {
		return nil, err
	}
	out := []*Host{}
	if hs.Hosts != nil {
		for _, raw := range hs.Hosts {
			h, err := HostFromJson(raw, getter)
			if err != nil {
				return nil, err
			}
			out = append(out, h)
		}
	}
	return out, nil
}

func HostFromJson(in []byte, getter plugin.SpecGetter) (*Host, error) {
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
			l, err := LocationFromJson(raw, getter)
			if err != nil {
				return nil, err
			}
			out.Locations = append(out.Locations, l)
		}
	}
	return out, nil
}

func LocationFromJson(in []byte, getter plugin.SpecGetter) (*Location, error) {
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
	for _, el := range l.Middlewares {
		m, err := MiddlewareFromJson(el, getter)
		if err != nil {
			return nil, err
		}
		loc.Middlewares = append(loc.Middlewares, m)
	}
	return loc, nil
}

func LocationOptionsFromJson(in []byte) (*LocationOptions, error) {
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

func MiddlewareFromJson(in []byte, getter plugin.SpecGetter) (*MiddlewareInstance, error) {
	var ms *rawMiddleware
	err := json.Unmarshal(in, &ms)
	if err != nil {
		return nil, err
	}
	spec := getter(ms.Type)
	if spec == nil {
		return nil, fmt.Errorf("Middleware of type %s is not supported", ms.Type)
	}
	m, err := spec.FromJson(ms.Middleware)
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

func UpstreamFromJson(in []byte) (*Upstream, error) {
	var u *Upstream
	err := json.Unmarshal(in, &u)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func EndpointFromJson(in []byte) (*Endpoint, error) {
	var e *Endpoint
	err := json.Unmarshal(in, &e)
	if err != nil {
		return nil, err
	}
	return NewEndpoint(e.UpstreamId, e.Id, e.Url)
}
