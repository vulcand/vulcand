package exproute

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"regexp"
	"strings"
)

// Regular expression to match url parameters
var reParam *regexp.Regexp

func init() {
	reParam = regexp.MustCompile("^<([^/]+)>")
}

// Trie http://en.wikipedia.org/wiki/Trie for url matching with support of named parameters
type trie struct {
	root *trieNode
}

// Takes the expression with url and the node that corresponds to this expression and returns parsed trie
func parseTrie(expression string, reqMatcher matcher) (*trie, error) {
	t := &trie{
		root: &trieNode{},
	}
	if len(expression) == 0 {
		return nil, fmt.Errorf("Empty URL expression")
	}
	err := t.root.parseExpression(-1, expression, reqMatcher)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Tries can merge with other tries
func (t *trie) canMerge(m matcher) bool {
	_, ok := m.(*trie)
	return ok
}

// Merge takes the other trie and modifies itself to match the passed trie as well.
// Note that trie passed as a parameter can be only simple trie without multiple branches per node, e.g. a->b->c->
// Trie on the left is "accumulating" trie that grows.
func (p *trie) merge(m matcher) (matcher, error) {
	other, ok := m.(*trie)
	if !ok {
		return nil, fmt.Errorf("Can't merge %T and %T")
	}
	root, err := p.root.merge(other.root)
	if err != nil {
		return nil, err
	}
	return &trie{root: root}, nil
}

// Takes the request and returns the location if the request path matches any of it's paths
// returns nil if none of the requests matches
func (p *trie) match(r request.Request) location.Location {
	if p.root == nil {
		return nil
	}

	path := r.GetHttpRequest().URL.Path
	if len(path) == 0 {
		path = "/"
	}
	return p.root.match(-1, path, r)
}

type trieNode struct {
	// Matching character, can be empty in case if it's a root node
	// or node with a pattern matcher
	char byte
	// Optional children of this node, can be empty if it's a leaf node
	children []*trieNode
	// If present, means that this node is a pattern matcher
	patternMatcher patternMatcher
	// If present it means this node contains potential match for a request, and this is a leaf node.
	requestMatchers []matcher
}

func (e *trieNode) isMatching() bool {
	return len(e.requestMatchers) != 0
}

func (e *trieNode) isRoot() bool {
	return e.char == byte(0) && e.patternMatcher == nil
}

func (e *trieNode) isPatternMatcher() bool {
	return e.patternMatcher != nil
}

func (e *trieNode) isCharMatcher() bool {
	return e.char != 0
}

func (e *trieNode) String() string {
	self := ""
	if e.patternMatcher != nil {
		self = e.patternMatcher.String()
	} else {
		self = fmt.Sprintf("%c", e.char)
	}
	if e.isMatching() {
		return fmt.Sprintf("match(%s)", self)
	} else if e.isRoot() {
		return fmt.Sprintf("root")
	} else {
		return fmt.Sprintf("node(%s)", self)
	}
}

func (e *trieNode) equals(o *trieNode) bool {
	return (e.char == o.char) &&
		(e.patternMatcher == nil && o.patternMatcher == nil) || // both nodes have no matchers
		((e.patternMatcher != nil && o.patternMatcher != nil) && e.patternMatcher.equals(o.patternMatcher)) // both nodes have equal matchers
}

func (e *trieNode) merge(o *trieNode) (*trieNode, error) {
	children := make([]*trieNode, 0, len(e.children))
	merged := make(map[*trieNode]bool)

	// First, find the nodes with similar keys and merge them
	for _, c := range e.children {
		for _, c2 := range o.children {
			// The nodes are equivalent, so we can merge them
			if c.equals(c2) {
				m, err := c.merge(c2)
				if err != nil {
					return nil, err
				}
				merged[c] = true
				merged[c2] = true
				children = append(children, m)
			}
		}
	}

	// Next, append the keys that haven't been merged
	for _, c := range e.children {
		if !merged[c] {
			children = append(children, c)
		}
	}

	for _, c := range o.children {
		if !merged[c] {
			children = append(children, c)
		}
	}

	return &trieNode{
		char:            e.char,
		children:        children,
		patternMatcher:  e.patternMatcher,
		requestMatchers: append(e.requestMatchers, o.requestMatchers...),
	}, nil
}

func (p *trieNode) parseExpression(offset int, pattern string, requestMatcher matcher) error {
	// We are the last element, so we are the matching node
	if offset >= len(pattern)-1 {
		p.requestMatchers = []matcher{requestMatcher}
		return nil
	}

	// There's a next character that exists
	patternMatcher, newOffset, err := parsePatternMatcher(offset+1, pattern)
	// We have found the matcher, but the syntax or parameters are wrong
	if err != nil {
		return err
	}
	// Matcher was found
	if patternMatcher != nil {
		node := &trieNode{patternMatcher: patternMatcher}
		p.children = []*trieNode{node}
		return node.parseExpression(newOffset-1, pattern, requestMatcher)
	} else {
		// Matcher was not found, next node is just a character
		node := &trieNode{char: pattern[offset+1]}
		p.children = []*trieNode{node}
		return node.parseExpression(offset+1, pattern, requestMatcher)
	}
}

func mergeLeafs(a *trieNode, b *trieNode) (*trieNode, error) {
	if len(a.children) != 0 || len(b.children) != 0 {
		return nil, fmt.Errorf("Can't merge matching nodes with children")
	}
	matchers := make([]matcher, 0, len(a.requestMatchers)+len(b.requestMatchers))
	matchers = append(matchers, a.requestMatchers...)
	matchers = append(matchers, b.requestMatchers...)
	return &trieNode{
		char:            a.char,
		children:        a.children,
		patternMatcher:  a.patternMatcher,
		requestMatchers: matchers,
	}, nil
}

func mergeWithLeaf(base *trieNode, leaf *trieNode) (*trieNode, error) {
	n := &trieNode{
		char:            base.char,
		children:        make([]*trieNode, len(base.children)),
		patternMatcher:  base.patternMatcher,
		requestMatchers: leaf.requestMatchers,
	}
	copy(n.children, base.children)
	return n, nil
}

func parsePatternMatcher(offset int, pattern string) (patternMatcher, int, error) {
	if pattern[offset] != '<' {
		return nil, -1, nil
	}
	rest := pattern[offset:]
	match := reParam.FindStringSubmatchIndex(rest)
	if len(match) == 0 {
		return nil, -1, nil
	}
	// Split parsed matcher parameters separated by :
	values := strings.Split(rest[match[2]:match[3]], ":")

	// The common syntax is <matcherType:matcherArg1:matcherArg2>
	matcherType := values[0]
	matcherArgs := values[1:]

	// In case if there's only one  <param> is implicitly converted to <string:param>
	if len(values) == 1 {
		matcherType = "string"
		matcherArgs = values
	}

	matcher, err := makePathMatcher(matcherType, matcherArgs)
	if err != nil {
		return nil, offset, err
	}
	return matcher, offset + match[1], nil
}

type matchResult struct {
	matcher patternMatcher
	value   interface{}
}

type patternMatcher interface {
	getName() string
	match(offset int, path string) (*matchResult, int)
	equals(other patternMatcher) bool
	String() string
}

func makePathMatcher(matcherType string, matcherArgs []string) (patternMatcher, error) {
	switch matcherType {
	case "string":
		return newStringMatcher(matcherArgs)
	}
	return nil, fmt.Errorf("Unsupported matcher: %s", matcherType)
}

func newStringMatcher(args []string) (patternMatcher, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Expected only one parameter - variable name, got %s", args)
	}
	return &stringMatcher{name: args[0]}, nil
}

type stringMatcher struct {
	name string
}

func (s *stringMatcher) String() string {
	return fmt.Sprintf("<string:%s>", s.name)
}

func (s *stringMatcher) getName() string {
	return s.name
}

func (s *stringMatcher) match(offset int, path string) (*matchResult, int) {
	value, offset := grabValue(offset, path)
	return &matchResult{matcher: s, value: value}, offset
}

func (s *stringMatcher) equals(other patternMatcher) bool {
	_, ok := other.(*stringMatcher)
	return ok && other.getName() == s.getName()
}

func (e *trieNode) matchNode(offset int, path string) (bool, int) {
	// We are out of bounds
	if offset > len(path)-1 {
		return false, -1
	}
	if offset == -1 || (e.isCharMatcher() && e.char == path[offset]) {
		return true, offset + 1
	}
	if e.isPatternMatcher() {
		result, newOffset := e.patternMatcher.match(offset, path)
		if result != nil {
			return true, newOffset
		}
	}
	return false, -1
}

func (e *trieNode) match(offset int, path string, r request.Request) location.Location {
	matched, newOffset := e.matchNode(offset, path)
	if !matched {
		return nil
	}
	// This is a leaf node and we are at the last character of the pattern
	if len(e.requestMatchers) != 0 && newOffset == len(path) {
		for _, matcher := range e.requestMatchers {
			if l := matcher.match(r); l != nil {
				return l
			}
		}
	}
	// Check for the match in child nodes
	for _, c := range e.children {
		if loc := c.match(newOffset, path, r); loc != nil {
			return loc
		}
	}
	return nil
}

// Grabs value until separator or next string
func grabValue(offset int, path string) (string, int) {
	rest := path[offset:]
	index := strings.Index(rest, "/")
	if index == -1 {
		return rest, len(path)
	}
	return rest[:index], offset + index
}
