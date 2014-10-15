/*
Expression based request router, supports functions and combinations of functions in form

<What to match><Matching verb> and || and && operators.

*/
package exproute

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

type ExpRouter struct {
	mutex    *sync.RWMutex
	matchers []matcher
	routes   map[string]location.Location
}

func NewExpRouter() *ExpRouter {
	return &ExpRouter{
		mutex:  &sync.RWMutex{},
		routes: make(map[string]location.Location),
	}
}

func (e *ExpRouter) GetLocationByExpression(expr string) location.Location {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.routes[expr]
}

func (e *ExpRouter) AddLocation(expr string, l location.Location) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if _, ok := e.routes[expr]; ok {
		return fmt.Errorf("Expression '%s' already exists", expr)
	}
	if _, err := parseExpression(expr, l); err != nil {
		return err
	}
	e.routes[expr] = l
	if err := e.compile(); err != nil {
		delete(e.routes, expr)
		return err
	}
	return nil
}

func (e *ExpRouter) compile() error {
	var exprs = []string{}
	for expr, _ := range e.routes {
		exprs = append(exprs, expr)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(exprs)))

	matchers := []matcher{}
	i := 0
	for _, expr := range exprs {
		location := e.routes[expr]
		matcher, err := parseExpression(expr, location)
		if err != nil {
			return err
		}

		// Merge the previous and new matcher if that's possible
		if i > 0 && matchers[i-1].canMerge(matcher) {
			m, err := matchers[i-1].merge(matcher)
			if err != nil {
				return err
			}
			matchers[i-1] = m
		} else {
			matchers = append(matchers, matcher)
			i += 1
		}
	}

	e.matchers = matchers
	return nil
}

func (e *ExpRouter) RemoveLocationByExpression(expr string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.routes, expr)
	return e.compile()
}

func (e *ExpRouter) RemoveLocationById(id string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for expr, l := range e.routes {
		if l.GetId() == id {
			delete(e.routes, expr)
		}
	}
	return e.compile()
}

func (e *ExpRouter) GetLocationById(id string) location.Location {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	for _, l := range e.routes {
		if l.GetId() == id {
			return l
		}
	}
	return nil
}

func (e *ExpRouter) Route(req request.Request) (location.Location, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if len(e.matchers) == 0 {
		return nil, nil
	}

	for _, m := range e.matchers {
		if l := m.match(req); l != nil {
			return l, nil
		}
	}
	return nil, nil
}
