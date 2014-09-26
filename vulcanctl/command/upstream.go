package command

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
					cli.StringFlag{Name: "id", Usage: "upstream id"},
				},
			},
			{
				Name:   "rm",
				Usage:  "Remove upstream from vulcan",
				Action: cmd.deleteUpstreamAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "upstream id"},
				},
			},
			{
				Name:   "ls",
				Usage:  "List upstreams",
				Action: cmd.listUpstreamsAction,
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
