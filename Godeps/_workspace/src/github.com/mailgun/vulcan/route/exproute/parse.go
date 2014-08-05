package exproute

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/location"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// Parses expression in the go language into matchers, e.g.
// `TrieRoute("/path")` will be parsed into trie matcher
// Enforces expression to use only registered functions and string literals
func parseExpression(in string, l location.Location) (matcher, error) {
	expr, err := parser.ParseExpr(in)
	if err != nil {
		return nil, err
	}

	var matcher matcher
	matcher = &constMatcher{location: l}
	var call *funcCall

	ast.Inspect(expr, func(n ast.Node) bool {
		// If error condition has been triggered, stop inspecting.
		if err != nil {
			return false
		}
		switch x := n.(type) {
		case *ast.BasicLit:
			if call == nil {
				err = fmt.Errorf("Literals are supported only as function arguments")
				return false
			}
			err = addFunctionArgument(call, x)
		case *ast.CallExpr:
			if call != nil {
				err = fmt.Errorf("Nested function calls are not allowed")
				return false
			}
			call = &funcCall{}
		case *ast.Ident:
			if call == nil {
				err = fmt.Errorf("Unsupported identifier")
				return false
			}
			call.name = x.Name
		default:
			if x != nil {
				err = fmt.Errorf("Unsupported %T", n)
				return false
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return createMatcher(matcher, call)
}

func addFunctionArgument(call *funcCall, a *ast.BasicLit) error {
	if a.Kind != token.STRING {
		return fmt.Errorf("Only string literals are supported as function arguments")
	}
	value, err := strconv.Unquote(a.Value)
	if err != nil {
		return fmt.Errorf("Failed to parse argument: %s, error: %s", a.Value, err)
	}
	call.args = append(call.args, value)
	return nil
}

func createMatcher(currentMatcher matcher, call *funcCall) (matcher, error) {
	switch call.name {
	case TrieRouteFn:
		return makeTrieRouteMatcher(currentMatcher, call.args)
	case RegexpRouteFn:
		return makeRegexpRouteMatcher(currentMatcher, call.args)
	}
	return nil, fmt.Errorf("Unsupported method: %s", call.name)
}

type funcCall struct {
	name string
	args []interface{}
}

func makeTrieRouteMatcher(matcher matcher, params []interface{}) (matcher, error) {
	if len(params) <= 0 {
		return nil, fmt.Errorf("%s accepts at least one argument - path to match", TrieRouteFn)
	}
	args, err := toStrings(params)
	if err != nil {
		return nil, err
	}

	// The first 0..n-1 arguments are considered to be request methods, e.g. (POST|GET|DELETE)
	if len(args) > 1 {
		matcher = &methodMatcher{methods: args[:len(args)-1], matcher: matcher}
	}

	t, err := parseTrie(args[len(args)-1], matcher)
	if err != nil {
		return nil, fmt.Errorf("%s - failed to parse path expression, %s", err)
	}
	return t, nil
}

func makeRegexpRouteMatcher(matcher matcher, params []interface{}) (matcher, error) {
	if len(params) <= 0 {
		return nil, fmt.Errorf("%s needs at least one argument - path to match", RegexpRouteFn)
	}
	args, err := toStrings(params)
	if err != nil {
		return nil, err
	}

	// The first 0..n-1 arguments are considered to be request methods, e.g. (POST|GET|DELETE)
	if len(args) > 1 {
		matcher = &methodMatcher{methods: args[:len(args)-1], matcher: matcher}
	}

	t, err := newRegexpMatcher(args[len(args)-1], mapRequestToPath, matcher)
	if err != nil {
		return nil, fmt.Errorf("Error %s(%s) - %s", RegexpRouteFn, params, err)
	}
	return t, nil
}

func toStrings(in []interface{}) ([]string, error) {
	out := make([]string, len(in))
	for i, v := range in {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("Expected string, got %T", v)
		}
		out[i] = s
	}
	return out, nil
}

const (
	TrieRouteFn   = "TrieRoute"
	RegexpRouteFn = "RegexpRoute"
)
