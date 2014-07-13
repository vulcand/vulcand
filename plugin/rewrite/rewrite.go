package rewrite

import (
	"net/http"
	"regexp"

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
	newPath     []byte
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
	oldPath := r.GetHttpRequest().URL.Path
	rewrite.newPath = rewrite.regexp.ReplaceAll([]byte(oldPath), []byte(rewrite.replacement))
	r.GetHttpRequest().URL.Path = string(rewrite.newPath)
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
		cli.StringFlag{"regexp", "", "regex to match against http request path"},
		cli.StringFlag{"replacement", "", "replacement text into which regex expansions are inserted"},
	}
}
