package main

import (
	"github.com/mailgun/cli"
)

func NewHostCommand() cli.Command {
	return cli.Command{
		Name:  "host",
		Usage: "Operations with vulcan hosts",
	}
}

func NewHostSubcommands() []cli.Command {
	return []cli.Command{
		{
			Name: "add",
			Flags: []cli.Flag{
				cli.StringFlag{"name", "", "hostname"},
			},
			Usage:  "Add a new host to vulcan proxy",
			Action: addHostAction,
		},
		{
			Name: "rm",
			Flags: []cli.Flag{
				cli.StringFlag{"name", "", "hostname"},
			},
			Usage:  "Remove a host from vulcan",
			Action: deleteHostAction,
		},
	}
}

func addHostAction(c *cli.Context) {
	printStatus(client(c).AddHost(c.String("name")))
}

func deleteHostAction(c *cli.Context) {
	printStatus(client(c).DeleteHost(c.String("name")))
}
