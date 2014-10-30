package plugin

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
)

// Middleware specification, used to construct new middlewares and plug them
// into CLI API and backends
type MiddlewareSpec struct {
	Type string
	// Reader function that returns a middleware factory from another
	// middleware structure
	FromOther interface{}
	// Flags for CLI tool to generate interface
	CliFlags []cli.Flag
	// Function that construtcs a middleware from CLI parameters
	FromCli CliReader
}

func (ms *MiddlewareSpec) FromJSON(data []byte) (MiddlewareFactory, error) {
	// Get a function's type
	fnType := reflect.TypeOf(ms.FromOther)

	// Create a pointer to the function's first argument
	ptr := reflect.New(fnType.In(0)).Interface()
	err := json.Unmarshal(data, &ptr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode %T from JSON, error: %s", ptr, err)
	}
	// Now let's call the function to produce a middleware
	fnVal := reflect.ValueOf(ms.FromOther)
	results := fnVal.Call([]reflect.Value{reflect.ValueOf(ptr).Elem()})

	m, out := results[0].Interface(), results[1].Interface()
	if out != nil {
		return nil, out.(error)
	}
	return m.(MiddlewareFactory), nil
}

type MiddlewareFactory interface {
	// Returns vulcan library compatible middleware
	NewMiddleware() (middleware.Middleware, error)
}

// Reader constructs a middleware factory from the CLI interface
type CliReader func(c *cli.Context) (MiddlewareFactory, error)

// Function that returns middleware spec by it's type
type SpecGetter func(string) *MiddlewareSpec

// Registry contains currently registered middlewares and used to support pluggable middlewares across all modules of the vulcand
type Registry struct {
	specs []*MiddlewareSpec
}

func NewRegistry() *Registry {
	return &Registry{
		specs: []*MiddlewareSpec{},
	}
}

func (r *Registry) AddSpec(s *MiddlewareSpec) error {
	if s == nil {
		return fmt.Errorf("spec can not be nil")
	}
	if r.GetSpec(s.Type) != nil {
		return fmt.Errorf("middleware of type %s already registered", s.Type)
	}
	if err := verifySignature(s.FromOther); err != nil {
		return err
	}
	r.specs = append(r.specs, s)
	return nil
}

func (r *Registry) GetSpec(middlewareType string) *MiddlewareSpec {
	for _, s := range r.specs {
		if s.Type == middlewareType {
			return s
		}
	}
	return nil
}

func (r *Registry) GetSpecs() []*MiddlewareSpec {
	return r.specs
}

func verifySignature(fn interface{}) error {
	t := reflect.TypeOf(fn)
	if t == nil || t.Kind() != reflect.Func {
		return fmt.Errorf("expected function, got %s", t)
	}
	if t.NumIn() != 1 {
		return fmt.Errorf("expected function with one input argument, got %d", t.NumIn())
	}
	if t.In(0).Kind() != reflect.Struct {
		return fmt.Errorf("function argument should be struct, got %s", t.In(0).Kind())
	}
	if t.NumOut() != 2 {
		return fmt.Errorf("function should return 2 values, got %d", t.NumOut())
	}
	if !t.Out(0).AssignableTo(reflect.TypeOf((*MiddlewareFactory)(nil)).Elem()) {
		return fmt.Errorf("function first return value should be Middleware got, %s", t.Out(0))
	}
	if !t.Out(1).AssignableTo(reflect.TypeOf((*error)(nil)).Elem()) {
		return fmt.Errorf("function second return value should be error got, %s", t.Out(1))
	}
	return nil
}
