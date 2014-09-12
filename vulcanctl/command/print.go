package command

import (
	"fmt"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/wsxiaoys/terminal/color"
	"github.com/mailgun/vulcand/backend"
)

func (cmd *Command) printResult(format string, in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOk(format, formatInstance(in))
	}
}

func (cmd *Command) printStatus(in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOk("%s", in)
	}
}

func (cmd *Command) printError(err error) {
	color.Fprint(cmd.out, fmt.Sprintf("@rERROR: %s\n", err))
}

func (cmd *Command) printOk(message string, params ...interface{}) {
	color.Fprint(cmd.out, fmt.Sprintf("@gOK: %s\n", fmt.Sprintf(message, params...)))
}

func (cmd *Command) printInfo(message string, params ...interface{}) {
	color.Fprint(cmd.out, "INFO: @w%s\n", fmt.Sprintf(message, params...))
}

func (cmd *Command) printHosts(hosts []*backend.Host) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, &VulcanTree{root: hosts}, 0, true, "")
}

func (cmd *Command) printUpstreams(upstreams []*backend.Upstream) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, &VulcanTree{root: upstreams}, 0, true, "")
}

func formatInstance(in interface{}) string {
	switch r := in.(type) {
	case *backend.Host:
		return fmt.Sprintf("host[name=%s]", r.Name)
	case *backend.Upstream:
		return fmt.Sprintf("upstream[id=%s]", r.Id)
	case *backend.Endpoint:
		if r.Stats != nil {
			s := r.Stats
			reqsSec := (s.Failures + s.Successes) / int64(s.PeriodSeconds)
			return fmt.Sprintf("endpoint[id=%s, url=%s, %d requests/sec, %.2f failures/sec]", r.Id, r.Url, reqsSec, s.FailRate)
		}
		return fmt.Sprintf("endpoint[id=%s, url=%s]", r.Id, r.Url)
	case *backend.Location:
		return fmt.Sprintf("location[id=%s, path=%s]", r.Id, r.Path)
	case *backend.MiddlewareInstance:
		return fmt.Sprintf("%s[id=%s, priority=%d, %s]", r.Type, r.Id, r.Priority, r.Middleware)
	}
	return fmt.Sprintf("%s", in)
}
