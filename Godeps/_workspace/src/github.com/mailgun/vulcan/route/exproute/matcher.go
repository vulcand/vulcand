package exproute

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"regexp"
)

type matcher interface {
	// Tells if matcher can be effectively merged with another matcher
	// (e.g. one trie can be merged with another trie)
	canMerge(matcher) bool
	// Merges this matcher with another matcher, modifies the orignal matcher.
	merge(matcher) (matcher, error)
	// Takes the request and returns attached location if the request matches.
	match(req request.Request) location.Location
}

// Maps request to some string, e.g. Request -> Path
type requestMapper func(req request.Request) string

// Maps request to its path
func mapRequestToPath(req request.Request) string {
	return req.GetHttpRequest().URL.Path
}

// Regular expression matcher, takes a regular expression and requestMapper
type regexpMatcher struct {
	// Uses this mapper to extract a string from a request to match agains
	mapper requestMapper
	// Compiled regular expression
	expr *regexp.Regexp
	// Inner matcher, will be executed if the regular expression passes.
	// This inner matcher can be a constMatcher if we need to match only regular expression
	// or some other matcher if we need to match request against several predicates at once,
	// e.g. Path and Method at the same time. Having a matcher within a matcher allows us to
	// chain matchers one by one.
	matcher matcher
}

func newRegexpMatcher(expr string, mapper requestMapper, matcher matcher) (matcher, error) {
	r, err := regexp.Compile(expr)

	if err != nil {
		return nil, fmt.Errorf("Bad regular expression: %s %s", expr, err)
	}
	return &regexpMatcher{expr: r, mapper: mapper, matcher: matcher}, nil
}

func (m *regexpMatcher) canMerge(matcher) bool {
	return false
}

func (m *regexpMatcher) merge(matcher) (matcher, error) {
	return nil, fmt.Errorf("Method not supported")
}

func (m *regexpMatcher) match(req request.Request) location.Location {
	if m.expr.MatchString(m.mapper(req)) {
		return m.matcher.match(req)
	}
	return nil
}

// Const matcher matches all the requests and returns it's internal location.
// Think about it a leaf node in the match chain that returns a result.
type constMatcher struct {
	location location.Location
}

func (c *constMatcher) canMerge(matcher) bool {
	return false
}

func (c *constMatcher) merge(matcher) (matcher, error) {
	return nil, fmt.Errorf("Method not supported")
}

func (c *constMatcher) match(req request.Request) location.Location {
	return c.location
}

// Matches request by it's method, supports several methods and treats them as OR
type methodMatcher struct {
	methods []string
	matcher matcher
}

func (m *methodMatcher) canMerge(matcher) bool {
	return false
}

func (m *methodMatcher) merge(matcher) (matcher, error) {
	return nil, fmt.Errorf("Method not supported")
}

func (m *methodMatcher) match(req request.Request) location.Location {
	for _, c := range m.methods {
		if req.GetHttpRequest().Method == c {
			return m.matcher.match(req)
		}
	}
	return nil
}
