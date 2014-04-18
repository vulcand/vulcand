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
		{
			Name:  "drain",
			Usage: "Wait till there are no more connections for endpoints in the upstream",
			Flags: []cli.Flag{
				cli.StringFlag{"id", "", "upstream id"},
				cli.IntFlag{"timeout", 5, "timeout in seconds"},
			},
			Action: upstreamDrainConnections,
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

func upstreamDrainConnections(c *cli.Context) {
	connections, err := client(c).DrainUpstreamConnections(c.String("id"), c.String("timeout"))
	if err != nil {
		printError(err)
		return
	}
	if connections == 0 {
		printOk("Connections: %d", connections)
	} else {
		printInfo("Connections: %d", connections)
	}
}
