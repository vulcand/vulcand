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
			Flags: []cli.Flag{
				cli.StringFlag{"id", "", "upstream id"},
			},
		},
		{
			Name:   "rm",
			Usage:  "Remove upstream from vulcan",
			Action: deleteUpstreamAction,
			Flags: []cli.Flag{
				cli.StringFlag{"id", "", "upstream id"},
			},
		},
		{
			Name:   "ls",
			Usage:  "List upstreams",
			Action: listUpstreamsAction,
		},
	}
}

func addUpstreamAction(c *cli.Context) {
	printStatus(client(c).AddUpstream(c.String("id")))
}

func deleteUpstreamAction(c *cli.Context) {
	printStatus(client(c).DeleteUpstream(c.String("id")))
}

func listUpstreamsAction(c *cli.Context) {
	out, err := client(c).GetUpstreams()
	if err != nil {
		printError(err)
	} else {
		printUpstreams(out)
	}
}
