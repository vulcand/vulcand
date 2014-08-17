package failover

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
)

// Parses expression in the go language into Failover predicates
func ParseExpression(in string) (Predicate, error) {
	expr, err := parser.ParseExpr(in)
	if err != nil {
		return nil, err
	}

	return parseNode(expr)
}

func parseNode(node ast.Node) (Predicate, error) {
	switch n := node.(type) {
	case *ast.BinaryExpr:
		x, err := parseNode(n.X)
		if err != nil {
			return nil, err
		}
		y, err := parseNode(n.Y)
		if err != nil {
			return nil, err
		}
		return joinPredicates(n.Op, x, y)
	case *ast.Ident:
		return getPredicateByName(n.Name)
	case *ast.CallExpr:
		// We expect function that will return predicate
		name, err := getIdentifier(n.Fun)
		if err != nil {
			return nil, err
		}
		fn, err := getFunctionByName(name)
		if err != nil {
			return nil, err
		}
		arguments, err := collectLiterals(n.Args)
		if err != nil {
			return nil, err
		}
		return createPredicate(fn, arguments)
	case *ast.ParenExpr:
		return parseNode(n.X)
	}
	return nil, fmt.Errorf("unsupported %T", node)
}

func getIdentifier(node ast.Node) (string, error) {
	id, ok := node.(*ast.Ident)
	if !ok {
		return "", fmt.Errorf("expected identifier, got: %T", node)
	}
	return id.Name, nil
}

func collectLiterals(nodes []ast.Expr) ([]interface{}, error) {
	out := make([]interface{}, len(nodes))
	for i, n := range nodes {
		l, ok := n.(*ast.BasicLit)
		if !ok {
			return nil, fmt.Errorf("expected literal, got %T", n)
		}
		val, err := literalToValue(l)
		if err != nil {
			return nil, err
		}
		out[i] = val
	}
	return out, nil
}

func literalToValue(a *ast.BasicLit) (interface{}, error) {
	switch a.Kind {
	case token.INT:
		value, err := strconv.Atoi(a.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument: %s, error: %s", a.Value, err)
		}
		return value, nil
	case token.STRING:
		value, err := strconv.Unquote(a.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument: %s, error: %s", a.Value, err)
		}
		return value, nil
	}
	return nil, fmt.Errorf("only integer and string literals are supported as function arguments")
}

func getPredicateByName(name string) (Predicate, error) {
	p, ok := map[string]Predicate{
		"IsNetworkError": IsNetworkError,
	}[name]
	if !ok {
		return nil, fmt.Errorf("unsupported predicate: %s", name)
	}
	return p, nil
}

func getFunctionByName(name string) (interface{}, error) {
	v, ok := map[string]interface{}{
		"RequestMethodEq": RequestMethodEq,
		"AttemptsLe":      AttemptsLe,
		"ResponseCodeEq":  ResponseCodeEq,
	}[name]
	if !ok {
		return nil, fmt.Errorf("unsupported method: %s", name)
	}
	return v, nil
}

func createPredicate(f interface{}, args []interface{}) (p Predicate, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s", r)
		}
	}()
	// This function can panic, that's why we are catching errors early
	return createPredicateInner(f, args)
}

func createPredicateInner(f interface{}, args []interface{}) (Predicate, error) {
	arguments := make([]reflect.Value, len(args))
	for i, a := range args {
		arguments[i] = reflect.ValueOf(a)
	}
	fn := reflect.ValueOf(f)

	ret := fn.Call(arguments)
	i := ret[0].Interface()
	return i.(Predicate), nil
}

func joinPredicates(op token.Token, a, b Predicate) (Predicate, error) {
	switch op {
	case token.LAND:
		return And(a, b), nil
	case token.LOR:
		return Or(a, b), nil
	}
	return nil, fmt.Errorf("unsupported operator: %s", op)
}
