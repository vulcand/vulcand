package command

import (
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcand/backend"
)

func NewListenerCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "listener",
		Usage: "Operations with vulcan listeners",
		Subcommands: []cli.Command{
			{
				Name:  "add",
				Usage: "Add a new listener to host",
				Flags: append([]cli.Flag{
					cli.StringFlag{Name: "id", Usage: "id, autogenerated if empty"},
					cli.StringFlag{Name: "host", Usage: "parent host"},
					cli.StringFlag{Name: "proto", Usage: "protocol, either http or https"},
					cli.StringFlag{Name: "net", Value: "tcp", Usage: "network, tcp or unix"},
					cli.StringFlag{Name: "addr", Value: "tcp", Usage: "address to bind to, e.g. 'localhost:31000'"},
				}, locationOptions()...),
				Action: cmd.addListenerAction,
			},
			{
				Name:   "rm",
				Usage:  "Remove a listener from host",
				Action: cmd.deleteListenerAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "id"},
					cli.StringFlag{Name: "host", Usage: "parent host"},
				},
			},
		},
	}
}

func (cmd *Command) addListenerAction(c *cli.Context) {
	listener, err := backend.NewListener(c.String("id"), c.String("proto"), c.String("net"), c.String("addr"))
	if err != nil {
		cmd.printError(err)
	}
	cmd.printStatus(cmd.client.AddHostListener(c.String("host"), listener))
}

func (cmd *Command) deleteListenerAction(c *cli.Context) {
	cmd.printStatus(cmd.client.DeleteHostListener(c.String("host"), c.String("id")))
}
