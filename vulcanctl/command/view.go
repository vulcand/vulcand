package command

import (
	"fmt"
	"sort"

	"github.com/mailgun/vulcand/backend"
)

// Detailed hosts view with all the information
func hostsView(hs []*backend.Host) *StringTree {
	r := &StringTree{
		Node: "[hosts]",
	}
	for _, h := range hs {
		r.AddChild(hostView(h))
	}
	return r
}

func hostView(h *backend.Host) *StringTree {
	host := &StringTree{
		Node: fmt.Sprintf("host[%s]", h.Name),
	}

	if len(h.Locations) != 0 {
		host.AddChild(locationsView(h.Locations))
	}

	if len(h.Listeners) != 0 {
		host.AddChild(listenersView(h.Listeners))
	}

	return host
}

func listenersView(ls []*backend.Listener) *StringTree {
	r := &StringTree{
		Node: "[listeners]",
	}
	if len(ls) == 0 {
		return r
	}
	for _, l := range ls {
		r.AddChild(
			&StringTree{
				Node: fmt.Sprintf("listener[%s, %s, %s://%s]", l.Id, l.Protocol, l.Address.Network, l.Address.Address),
			})
	}
	return r
}

func locationsView(ls []*backend.Location) *StringTree {
	r := &StringTree{
		Node: "[locations]",
	}
	if len(ls) == 0 {
		return r
	}
	sort.Sort(&locSorter{locs: ls})
	for _, l := range ls {
		r.AddChild(locationView(l))
	}
	return r
}

func locationView(l *backend.Location) *StringTree {
	r := &StringTree{
		Node: fmt.Sprintf("loc[%s, %s]", l.Id, l.Path),
	}

	// Display upstream information
	r.AddChild(upstreamView(l.Upstream))

	// Middlewares information
	if len(l.Middlewares) != 0 {
		r.AddChild(middlewaresView(l.Middlewares))
	}
	return r
}

func upstreamsView(us []*backend.Upstream) *StringTree {
	r := &StringTree{
		Node: "[upstreams]",
	}
	for _, u := range us {
		r.AddChild(upstreamView(u))
	}
	return r
}

func upstreamView(u *backend.Upstream) *StringTree {
	r := &StringTree{
		Node: fmt.Sprintf("upstream[%s]", u.Id),
	}

	for _, e := range u.Endpoints {
		r.AddChild(endpointView(e))
	}
	return r
}

func endpointView(e *backend.Endpoint) *StringTree {
	return &StringTree{
		Node: fmt.Sprintf("endpoint[%s, %s]", e.Id, e.Url),
	}
}

func middlewaresView(ms []*backend.MiddlewareInstance) *StringTree {
	r := &StringTree{
		Node: "[middlewares]",
	}
	if len(ms) == 0 {
		return r
	}
	sort.Sort(&middlewareSorter{ms: ms})
	for _, m := range ms {
		r.AddChild(middlewareView(m))
	}
	return r
}

func middlewareView(m *backend.MiddlewareInstance) *StringTree {
	return &StringTree{
		Node: fmt.Sprintf("%s[%d, %s, %s]", m.Type, m.Priority, m.Id, m.Middleware),
	}
}

// Sorts middlewares by their priority
type middlewareSorter struct {
	ms []*backend.MiddlewareInstance
}

// Len is part of sort.Interface.
func (s *middlewareSorter) Len() int {
	return len(s.ms)
}

// Swap is part of sort.Interface.
func (s *middlewareSorter) Swap(i, j int) {
	s.ms[i], s.ms[j] = s.ms[j], s.ms[i]
}

func (s *middlewareSorter) Less(i, j int) bool {
	return s.ms[i].Priority < s.ms[j].Priority
}
