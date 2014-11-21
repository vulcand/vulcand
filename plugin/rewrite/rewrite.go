package rewrite

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/template"
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
	return &RewriteInstance{re, replacement}, nil
}

func (rewrite *RewriteInstance) ProcessRequest(r request.Request) (*http.Response, error) {
	oldURL := r.GetHttpRequest().URL.String()

	// apply a rewrite regexp to the URL
	newURL := rewrite.regexp.ReplaceAllString(oldURL, rewrite.replacement)

	// replace any variables that may be in there
	newURL, err := template.Apply(newURL, template.Data{r.GetHttpRequest()})
	if err != nil {
		return nil, err
	}

	// parse the rewritten URL and replace request URL with it
	parsedURL, err := url.Parse(newURL)
	if err != nil {
		return nil, err
	}

	r.GetHttpRequest().URL = parsedURL

	return nil, nil
}

func (*RewriteInstance) ProcessResponse(r request.Request, a request.Attempt) {
	body, err := ioutil.ReadAll(a.GetResponse().Body)
	if err != nil {
		return
	}

	newBody, err := template.Apply(string(body), template.Data{r.GetHttpRequest()})
	if err != nil {
		return
	}

	a.GetResponse().Body = ioutil.NopCloser(strings.NewReader(newBody))
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
