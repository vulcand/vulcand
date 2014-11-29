package engine

import (
	"encoding/json"
	"fmt"

	"github.com/mailgun/vulcand/plugin"
)

type rawFrontends struct {
	Frontends []json.RawMessage
}

type rawFrontend struct {
	Id        string
	Type      string
	BackendId string
	Settings  json.RawMessage
}

type rawBackend struct {
	Id       string
	Type     string
	Settings json.RawMessage
}

type RawMiddleware struct {
	Id         string
	Type       string
	Priority   int
	Middleware json.RawMessage
}

func HostsFromJSON(in []byte) ([]Host, error) {
	var hs []Host
	err := json.Unmarshal(in, &hs)
	if err != nil {
		return nil, err
	}
	out := []Host{}
	if len(hs) != 0 {
		for _, raw := range hs {
			h, err := NewHost(raw.Name, raw.Options)
			if err != nil {
				return nil, err
			}
			out = append(out, *h)
		}
	}
	return out, nil
}

func FrontendsFromJSON(in []byte) ([]Frontend, error) {
	var rf *rawFrontends
	err := json.Unmarshal(in, &rf)
	if err != nil {
		return nil, err
	}
	out := make([]Frontend, len(rf.Frontends))
	for i, raw := range rf.Frontends {
		f, err := FrontendFromJSON(raw)
		if err != nil {
			return nil, err
		}
		out[i] = *f
	}
	return out, nil
}

func HostFromJSON(in []byte) (*Host, error) {
	var h *Host
	err := json.Unmarshal(in, &h)
	if err != nil {
		return nil, err
	}
	return NewHost(h.Name, h.Options)
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

func FrontendFromJSON(in []byte) (*Frontend, error) {
	var rf *rawFrontend
	if err := json.Unmarshal(in, &rf); err != nil {
		return nil, err
	}
	if rf.Type != HTTP {
		return nil, fmt.Errorf("Unsupported frontend type: %v", rf.Type)
	}
	var s *HTTPFrontendSettings
	if err := json.Unmarshal(rf.Settings, &s); err != nil {
		return nil, err
	}
	if s == nil {
		s = &HTTPFrontendSettings{}
	}
	f, err := NewHTTPFrontend(rf.Id, rf.BackendId, *s)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func MiddlewareFromJSON(in []byte, getter plugin.SpecGetter) (*Middleware, error) {
	var ms *RawMiddleware
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
	return &Middleware{
		Id:         ms.Id,
		Type:       ms.Type,
		Middleware: m,
		Priority:   ms.Priority,
	}, nil
}

func BackendFromJSON(in []byte) (*Backend, error) {
	var rb *rawBackend

	if err := json.Unmarshal(in, &rb); err != nil {
		return nil, err
	}
	if rb.Type != HTTP {
		return nil, fmt.Errorf("Unsupported backend type %v", rb.Type)
	}

	var s *HTTPBackendSettings
	if err := json.Unmarshal(rb.Settings, &s); err != nil {
		return nil, err
	}
	if s == nil {
		s = &HTTPBackendSettings{}
	}
	b, err := NewHTTPBackend(rb.Id, *s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ServerFromJSON(in []byte) (*Server, error) {
	var e *Server
	err := json.Unmarshal(in, &e)
	if err != nil {
		return nil, err
	}
	return NewServer(e.Id, e.URL)
}
