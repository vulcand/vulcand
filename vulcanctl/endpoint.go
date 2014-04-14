package main

import (
	"github.com/mailgun/cli"
)

func NewEndpointCommand() cli.Command {
	return cli.Command{
		Name:  "endpoint",
		Usage: "Operations with vulcan endpoint",
	}
}

func NewEndpointSubcommands() []cli.Command {
	return []cli.Command{
		{
			Name:   "add",
			Usage:  "Add a new endpoint to vulcan",
			Action: addEndpointAction,
		},
		{
			Name:   "rm",
			Usage:  "Remove endpoint from vulcan",
			Action: deleteEndpointAction,
		},
	}
}

func addEndpointAction(c *cli.Context) {
	err := client(c).AddEndpoint(c.Args().Get(0), c.Args().Get(1), c.Args().Get(2))
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
