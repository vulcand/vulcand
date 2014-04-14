package main

import (
	"github.com/mailgun/cli"
)

func NewUpstreamCommand() cli.Command {
	return cli.Command{
		Name:  "upstream",
		Flags: flags(),
		Usage: "Operations with vulcan upstreams",
	}
}

func NewUpstreamSubcommands() []cli.Command {
	return []cli.Command{
		{
			Name:   "add",
			Usage:  "Add a new upstream to vulcan",
			Action: addUpstreamAction,
		},
		{
			Name:   "rm",
			Usage:  "Remove upstream from vulcan",
			Action: deleteUpstreamAction,
		},
		{
			Name:   "list",
			Usage:  "List upstreams",
			Flags:  flags(),
			Action: listUpstreamsAction,
		},
	}
}

func addUpstreamAction(c *cli.Context) {
	err := client(c).AddUpstream(c.Args().First())
	if err != nil {
		printError(err)
	} else {
		printOk("Upstream added")
	}
}

func deleteUpstreamAction(c *cli.Context) {
	err := client(c).DeleteUpstream(c.Args().First())
	if err != nil {
		printError(err)
	} else {
		printOk("Upstream deleted")
	}
}

func listUpstreamsAction(c *cli.Context) {
	out, err := client(c).GetUpstreams()
	if err != nil {
		printError(err)
	} else {
		printUpstreams(out)
	}
}
