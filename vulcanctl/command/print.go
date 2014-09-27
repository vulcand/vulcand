package command

import (
	"bytes"
	"fmt"

	"github.com/buger/goterm"
	"github.com/mailgun/vulcand/backend"
)

func (cmd *Command) printResult(format string, in interface{}, err error) {
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printOk(format, fmt.Sprintf("%v", in))
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
	fmt.Fprint(cmd.out, goterm.Color(fmt.Sprintf("ERROR: %s", err), goterm.RED)+"\n")
}

func (cmd *Command) printOk(message string, params ...interface{}) {
	fmt.Fprintf(cmd.out, goterm.Color(fmt.Sprintf("OK: %s\n", fmt.Sprintf(message, params...)), goterm.GREEN)+"\n")
}

func (cmd *Command) printInfo(message string, params ...interface{}) {
	fmt.Fprintf(cmd.out, "INFO: %s\n", fmt.Sprintf(message, params...))
}

func (cmd *Command) printHosts(hosts []*backend.Host) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, hostsView(hosts), 0, true, "")
}

func (cmd *Command) printHost(host *backend.Host) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, hostView(host), 0, true, "")
}

func (cmd *Command) printOverview(hosts []*backend.Host, limit int) {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, "\n")
	fmt.Fprintf(out, hostsOverview(hosts, limit))
	fmt.Fprintf(cmd.out, out.String())
}

func (cmd *Command) printUpstreams(upstreams []*backend.Upstream) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, upstreamsView(upstreams), 0, true, "")
}

func (cmd *Command) printUpstream(upstream *backend.Upstream) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, upstreamView(upstream), 0, true, "")
}

func (cmd *Command) printLocation(l *backend.Location) {
	fmt.Fprintf(cmd.out, "\n")
	printTree(cmd.out, locationView(l), 0, true, "")
}
