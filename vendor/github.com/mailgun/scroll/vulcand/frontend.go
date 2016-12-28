package vulcand

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultFailoverPredicate = "(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"
	defaultPassHostHeader    = true
)

type frontendSpec struct {
	ID          string
	Host        string
	Path        string
	URLPath     string
	Methods     []string
	AppName     string
	Options     frontendOptions
	Middlewares []Middleware
}

type frontendOptions struct {
	FailoverPredicate string `json:"FailoverPredicate"`
	PassHostHeader    bool   `json:"PassHostHeader,omitempty"`
}

func (fo frontendOptions) spec() string {
	return fmt.Sprintf(`{"FailoverPredicate":"%s","PassHostHeader":%t}`, fo.FailoverPredicate, fo.PassHostHeader)
}

func newFrontendSpec(appname, host, path string, methods []string, middlewares []Middleware) *frontendSpec {
	path = normalizePath(path)
	return &frontendSpec{
		ID:      makeLocationID(methods, path),
		Host:    host,
		Methods: methods,
		URLPath: path,
		Path:    makeLocationPath(methods, path),
		AppName: appname,
		Options: frontendOptions{
			FailoverPredicate: defaultFailoverPredicate,
			PassHostHeader:    defaultPassHostHeader,
		},
		Middlewares: middlewares,
	}
}

func (fes *frontendSpec) spec() string {
	return fmt.Sprintf(`{"Type":"http","BackendId":"%s","Route":%s,"Settings":%s}`,
		fes.AppName, strconv.Quote(fes.Route()), fes.Options.spec())
}

func (fes *frontendSpec) Route() string {
	var methodExpr string
	if len(fes.Methods) == 1 {
		methodExpr = fmt.Sprintf(`Method("%s")`, fes.Methods[0])
	} else {
		methodExpr = fmt.Sprintf(`MethodRegexp("%s")`, strings.Join(fes.Methods, "|"))
	}
	return fmt.Sprintf(`Host("%s") && %s && Path("%s")`, fes.Host, methodExpr, fes.URLPath)
}

func makeLocationID(methods []string, path string) string {
	return strings.ToLower(strings.Replace(fmt.Sprintf("%v%v", strings.Join(methods, "."), path), "/", ".", -1))
}

func makeLocationPath(methods []string, path string) string {
	return fmt.Sprintf(`TrieRoute("%v", "%v")`, strings.Join(methods, `", "`), path)
}

// normalizePath converts a router path to the format understood by Vulcand.
//
// It does two things:
//  - Strips regular expression parts of path variables, i.e. turns "/v2/{id:[0-9]+}" into "/v2/{id}".
//  - Replaces curly brackets with angle brackets, i.e. turns "/v2/{id}" into "/v2/<id>".
func normalizePath(path string) string {
	path = regexp.MustCompile("(:[^}]+)").ReplaceAllString(path, "")
	return strings.Replace(strings.Replace(path, "{", "<", -1), "}", ">", -1)
}
