package main

import (
	"github.com/codegangsta/cli"
)

func NewListUpstreamsCommand() cli.Command {
	return cli.Command{
		Name:   "upstreams",
		Usage:  "List upstreams",
		Flags:  flags(),
		Action: listUpstreamsAction,
	}
}

func NewUpstreamCommand() cli.Command {
	return NewGroupCommand(GroupCommand{
		Name:  "upstream",
		Usage: "Operations with vulcan upstreams",
		Flags: flags(),
		Subcommands: []cli.Command{
			{
				Name:   "add",
				Flags:  flags(),
				Usage:  "Add a new upstream to vulcan",
				Action: addUpstreamAction,
			},
			{
				Name:   "rm",
				Flags:  flags(),
				Usage:  "Remove upstream from vulcan",
				Action: deleteUpstreamAction,
			},
		},
	})
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
