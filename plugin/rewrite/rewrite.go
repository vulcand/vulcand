package rewrite

import (
	"bytes"
	"net/http"
	"net/url"
	"regexp"
	"text/template"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/plugin"
)

const Type = "rewrite"

func GetSpec() *plugin.MiddlewareSpec {
	return &plugin.MiddlewareSpec{
		Type:      Type,
		FromOther: FromOther,
		FromCli:   FromCli,
		CliFlags:  CliFlags(),
	}
}

type Rewrite struct {
	Regexp      string
	Replacement string
}

type RewriteInstance struct {
	regexp      *regexp.Regexp
	replacement string
}

func (r *RewriteInstance) NewMiddleware() (middleware.Middleware, error) {
	return r, nil
}

func NewRewriteInstance(regex, replacement string) (*RewriteInstance, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	return &RewriteInstance{regexp: re, replacement: replacement}, nil
}

func (rewrite *RewriteInstance) ProcessRequest(r Request) (*http.Response, error) {
	oldURL := r.GetHttpRequest().URL.String()

	// apply a rewrite regexp to the URL
	newURL := rewrite.regexp.ReplaceAllString(oldURL, rewrite.replacement)

	// then make a template out of the new URL to replace any variables
	// that may be in there
	t, err := template.New("t").Parse(newURL)
	if err != nil {
		return nil, err
	}

	// template data includes http.Request object making all its properties/methods
	// available inside replacement string
	context := struct{ Request *http.Request }{r.GetHttpRequest()}

	var b bytes.Buffer
	if err := t.Execute(&b, context); err != nil {
		return nil, err
	}

	// parse the rewritten URL and replace request URL with it
	parsedURL, err := url.Parse(b.String())
	if err != nil {
		return nil, err
	}

	r.GetHttpRequest().URL = parsedURL
	return nil, nil
}

func (*RewriteInstance) ProcessResponse(r Request, a Attempt) {
}

func FromOther(r Rewrite) (plugin.Middleware, error) {
	return NewRewriteInstance(r.Regexp, r.Replacement)
}

func FromCli(c *cli.Context) (plugin.Middleware, error) {
	return NewRewriteInstance(c.String("regexp"), c.String("replacement"))
}

func CliFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "regexp", Usage: "regex to match against http request path"},
		cli.StringFlag{Name: "replacement", Usage: "replacement text into which regex expansions are inserted"},
	}
}
