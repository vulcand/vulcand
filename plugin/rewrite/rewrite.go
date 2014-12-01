package rewrite

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/errors"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/middleware"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/netutils"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/request"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/template"

	"github.com/mailgun/vulcand/plugin"
)

const Type = "rewrite"

type Rewrite struct {
	Regexp      string
	Replacement string
	RewriteBody bool
	Redirect    bool
}

func NewRewrite(regex, replacement string, rewriteBody, redirect bool) (*Rewrite, error) {
	return &Rewrite{regex, replacement, rewriteBody, redirect}, nil
}

func (rw *Rewrite) NewMiddleware() (middleware.Middleware, error) {
	return NewRewriteInstance(rw)
}

func (rw *Rewrite) String() string {
	return fmt.Sprintf("regexp=%v, replacement=%v, rewriteBody=%v, redirect=%v",
		rw.Regexp, rw.Replacement, rw.RewriteBody, rw.Redirect)
}

type RewriteInstance struct {
	regexp      *regexp.Regexp
	replacement string
	rewriteBody bool
	redirect    bool
}

func NewRewriteInstance(spec *Rewrite) (*RewriteInstance, error) {
	re, err := regexp.Compile(spec.Regexp)
	if err != nil {
		return nil, err
	}
	return &RewriteInstance{re, spec.Replacement, spec.RewriteBody, spec.Redirect}, nil
}

func (rw *RewriteInstance) ProcessRequest(r request.Request) (*http.Response, error) {
	oldURL := netutils.RawURL(r.GetHttpRequest())

	// apply a rewrite regexp to the URL
	newURL := rw.regexp.ReplaceAllString(oldURL, rw.replacement)

	// replace any variables that may be in there
	rewrittenURL := &bytes.Buffer{}
	if err := template.ApplyString(newURL, rewrittenURL, r.GetHttpRequest()); err != nil {
		return nil, err
	}

	// parse the rewritten URL and replace request URL with it
	parsedURL, err := url.Parse(rewrittenURL.String())
	if err != nil {
		return nil, err
	}

	if rw.redirect {
		return nil, &errors.RedirectError{URL: parsedURL}
	}

	r.GetHttpRequest().URL = parsedURL
	return nil, nil
}

func (rw *RewriteInstance) ProcessResponse(r request.Request, a request.Attempt) {
	if !rw.rewriteBody {
		return
	}

	body := a.GetResponse().Body
	defer body.Close()

	newBody := &bytes.Buffer{}
	if err := template.Apply(body, newBody, r.GetHttpRequest()); err != nil {
		log.Errorf("Failed to rewrite response body: %v", err)
		return
	}

	a.GetResponse().Body = ioutil.NopCloser(newBody)
}

func FromOther(rw Rewrite) (plugin.Middleware, error) {
	return NewRewrite(rw.Regexp, rw.Replacement, rw.RewriteBody, rw.Redirect)
}

func FromCli(c *cli.Context) (plugin.Middleware, error) {
	return NewRewrite(c.String("regexp"), c.String("replacement"), c.Bool("rewriteBody"), c.Bool("redirect"))
}

func GetSpec() *plugin.MiddlewareSpec {
	return &plugin.MiddlewareSpec{
		Type:      Type,
		FromOther: FromOther,
		FromCli:   FromCli,
		CliFlags:  CliFlags(),
	}
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
		cli.BoolFlag{
			Name:  "redirect",
			Usage: "if provided, request is redirected to the rewritten URL",
		},
	}
}
