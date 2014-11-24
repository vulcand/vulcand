package rewrite

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
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
	RewriteBody bool
}

type RewriteInstance struct {
	regexp      *regexp.Regexp
	replacement string
	rewriteBody bool
}

func NewRewriteInstance(regex, replacement string, rewriteBody bool) (*RewriteInstance, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	return &RewriteInstance{re, replacement, rewriteBody}, nil
}

func (rw *RewriteInstance) NewMiddleware() (middleware.Middleware, error) {
	return rw, nil
}

func (rw *RewriteInstance) ProcessRequest(r request.Request) (*http.Response, error) {
	oldURL := r.GetHttpRequest().URL.String()

	// apply a rewrite regexp to the URL
	newURL := rw.regexp.ReplaceAllString(oldURL, rw.replacement)

	// replace any variables that may be in there
	newURL, err := template.Apply(newURL, r.GetHttpRequest())
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

func (rw *RewriteInstance) ProcessResponse(r request.Request, a request.Attempt) {
	if rw.rewriteBody != true {
		return
	}

	body, err := ioutil.ReadAll(a.GetResponse().Body)
	if err != nil {
		return
	}

	newBody, err := template.Apply(string(body), r.GetHttpRequest())
	if err != nil {
		return
	}

	bodyBuffer, err := netutils.NewBodyBuffer(strings.NewReader(newBody))
	if err != nil {
		return
	}

	a.GetResponse().Body = bodyBuffer
}

func FromOther(rw Rewrite) (plugin.Middleware, error) {
	return NewRewriteInstance(rw.Regexp, rw.Replacement, rw.RewriteBody)
}

func FromCli(c *cli.Context) (plugin.Middleware, error) {
	return NewRewriteInstance(c.String("regexp"), c.String("replacement"), c.Bool("rewriteBody"))
}

func CliFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "regexp",
			Usage: "regex to match against http request path",
		},
		cli.StringFlag{
			Name:  "replacement",
			Usage: "replacement text into which regex expansions are inserted",
		},
		cli.BoolFlag{
			Name:  "rewriteBody",
			Usage: "if provided, response body is treated as as template and all variables in it are replaced",
		},
	}
}
