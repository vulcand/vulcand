package main

import (
	"github.com/codegangsta/cli"
)

func NewEndpointCommand() cli.Command {
	return NewGroupCommand(GroupCommand{
		Name:  "endpoint",
		Usage: "Operations with vulcan endpoint",
		Flags: flags(),
		Subcommands: []cli.Command{
			{
				Name:   "add",
				Flags:  flags(),
				Usage:  "Add a new endpoint to vulcan",
				Action: addEndpointAction,
			},
			{
				Name:   "rm",
				Flags:  flags(),
				Usage:  "Remove endpoint from vulcan",
				Action: deleteEndpointAction,
			},
		},
	})
}

func addEndpointAction(c *cli.Context) {
	err := client(c).AddEndpoint(c.Args().Get(0), c.Args().Get(1))
	if err != nil {
		printError(err)
	} else {
		printOk("Endpoint added")
	}
}

func deleteEndpointAction(c *cli.Context) {
	err := client(c).DeleteEndpoint(c.Args().Get(0), c.Args().Get(1))
	if err != nil {
		printError(err)
	} else {
		printOk("Endpoint deleted")
	}
}
