/*
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
	fn, args, matched := extractFunction(in)
	if !matched {
		return in
	}
	pathMatcher := ""
	if fn == "TrieRoute" {
		pathMatcher = "Path"
	} else {
		pathMatcher = "PathRegexp"
	}
	if len(args) == 1 {
		return fmt.Sprintf(`%s("%s")`, pathMatcher, args[0])
	}
	if len(args) == 2 {
		return fmt.Sprintf(`Method("%s") && %s("%s")`, args[0], pathMatcher, args[1])
	}
	path := args[len(args)-1]
	methods := args[0 : len(args)-1]
	return fmt.Sprintf(`MethodRegexp("%s") && %s("%s")`, strings.Join(methods, "|"), pathMatcher, path)
}

func extractFunction(f string) (string, []string, bool) {
	match := regexp.MustCompile(`(TrieRoute|RegexpRoute)\(([^\(\)]+)\)`).FindStringSubmatch(f)
	if len(match) != 3 {
		return "", nil, false
	}
	fn := match[1]
	args := strings.Split(match[2], ",")
	arguments := make([]string, len(args))
	for i, a := range args {
		arguments[i] = strings.Trim(a, " ,\"")
	}
	return fn, arguments, true
}
