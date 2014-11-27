/*
Expression based request router, supports functions and combinations of functions in form

see http://godoc.org/github.com/mailgun/route for documentation on the language

*/
package exproute

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/route"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
)

type ExpRouter struct {
	r route.Router
}

func NewExpRouter() *ExpRouter {
	return &ExpRouter{
		r: route.New(),
	}
}

func (e *ExpRouter) GetLocationByExpression(expr string) location.Location {
	v := e.r.GetRoute(convertPath(expr))
	if v == nil {
		return nil
	}
	return v.(location.Location)
}

func (e *ExpRouter) AddLocation(expr string, l location.Location) error {
	return e.r.AddRoute(convertPath(expr), l)
}

func (e *ExpRouter) RemoveLocationByExpression(expr string) error {
	return e.r.RemoveRoute(convertPath(expr))
}

func (e *ExpRouter) Route(req request.Request) (location.Location, error) {
	l, err := e.r.Route(req.GetHttpRequest())
	if err != nil {
		return nil, err
	}
	if l == nil {
		return nil, nil
	}
	return l.(location.Location), nil
}

// convertPath changes strings to structured format /hello -> RegexpRoute("/hello") and leaves structured strings unchanged.
func convertPath(in string) string {
	if !strings.Contains(in, "(") {
		return fmt.Sprintf(`PathRegexp(%#v)`, in)
	}
	// Regexp Route with one parameter
	match := regexp.MustCompile(`RegexpRoute\(\s*("[^"]+")\s*\)`).FindStringSubmatch(in)
	if len(match) == 2 {
		return fmt.Sprintf("PathRegexp(%s)", match[1])
	}

	// Regexp route with two parameters
	match = regexp.MustCompile(`RegexpRoute\(\s*("[^"]+")\s*,\s*("[^"]+")\s*\)`).FindStringSubmatch(in)
	if len(match) == 3 {
		return fmt.Sprintf("Method(%s) && PathRegexp(%s)", match[1], match[2])
	}

	// Trie route with one parameter
	match = regexp.MustCompile(`TrieRoute\(\s*("[^"]+")\s*\)`).FindStringSubmatch(in)
	if len(match) == 2 {
		return fmt.Sprintf("Path(%s)", match[1])
	}

	// Trie route with two parameters
	match = regexp.MustCompile(`TrieRoute\(\s*("[^"]+")\s*,\s*("[^"]+")\s*\)`).FindStringSubmatch(in)
	if len(match) == 3 {
		return fmt.Sprintf("Method(%s) && Path(%s)", match[1], match[2])
	}

	return in
}
