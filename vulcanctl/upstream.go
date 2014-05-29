package main

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
)

func NewUpstreamCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "upstream",
		Usage: "Operations with vulcan upstreams",
		Subcommands: []cli.Command{
			{
				Name:   "add",
				Usage:  "Add a new upstream to vulcan",
				Action: cmd.addUpstreamAction,
				Flags: []cli.Flag{
					cli.StringFlag{"id", "", "upstream id"},
				},
			},
			{
				Name:   "rm",
				Usage:  "Remove upstream from vulcan",
				Action: cmd.deleteUpstreamAction,
				Flags: []cli.Flag{
					cli.StringFlag{"id", "", "upstream id"},
				},
			},
			{
				Name:   "ls",
				Usage:  "List upstreams",
				Action: cmd.listUpstreamsAction,
			},
			{
				Name:  "drain",
				Usage: "Wait till there are no more connections for endpoints in the upstream",
				Flags: []cli.Flag{
					cli.StringFlag{"id", "", "upstream id"},
					cli.IntFlag{"timeout", 5, "timeout in seconds"},
				},
				Action: cmd.upstreamDrainConnections,
			},
		},
	}
}

func (cmd *Command) addUpstreamAction(c *cli.Context) {
	u, err := cmd.client.AddUpstream(c.String("id"))
	cmd.printResult("%s added", u, err)
}

func (cmd *Command) deleteUpstreamAction(c *cli.Context) {
	cmd.printStatus(cmd.client.DeleteUpstream(c.String("id")))
}

func (cmd *Command) listUpstreamsAction(c *cli.Context) {
	out, err := cmd.client.GetUpstreams()
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printUpstreams(out)
	}
}

func (cmd *Command) upstreamDrainConnections(c *cli.Context) {
	connections, err := cmd.client.DrainUpstreamConnections(c.String("id"), c.String("timeout"))
	if err != nil {
		cmd.printError(err)
		return
	}
	if connections == 0 {
		cmd.printOk("Connections: %d", connections)
	} else {
		cmd.printInfo("Connections: %d", connections)
	}
}
