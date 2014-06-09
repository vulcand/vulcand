package command

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
)

func NewHostCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "host",
		Usage: "Operations with vulcan hosts",
		Subcommands: []cli.Command{
			{
				Name: "add",
				Flags: []cli.Flag{
					cli.StringFlag{"name", "", "hostname"},
				},
				Usage:  "Add a new host to vulcan proxy",
				Action: cmd.addHostAction,
			},
			{
				Name: "rm",
				Flags: []cli.Flag{
					cli.StringFlag{"name", "", "hostname"},
				},
				Usage:  "Remove a host from vulcan",
				Action: cmd.deleteHostAction,
			},
		},
	}
}

func (cmd *Command) addHostAction(c *cli.Context) {
	host, err := cmd.client.AddHost(c.String("name"))
	cmd.printResult("%s added", host, err)
}

func (cmd *Command) deleteHostAction(c *cli.Context) {
	cmd.printStatus(cmd.client.DeleteHost(c.String("name")))
}
