package main

import (
	"github.com/mailgun/cli"
)

func NewLocationCommand() cli.Command {
	return cli.Command{
		Name:  "location",
		Usage: "Operations with vulcan locations",
	}
}

func NewLocationSubcommands() []cli.Command {
	return []cli.Command{
		{
			Name:   "add",
			Usage:  "Add a new location to vulcan proxy",
			Action: addLocationAction,
		},
		{
			Name:   "rm",
			Usage:  "Remove a location from vulcan",
			Action: deleteLocationAction,
		},
		{
			Name:   "set_upstream",
			Usage:  "Update upstream",
			Action: locationUpdateUpstreamAction,
		},
	}
}

func addLocationAction(c *cli.Context) {
	if err := client(c).AddLocation(c.Args().Get(0), c.Args().Get(1), c.Args().Get(2), c.Args().Get(3)); err != nil {
		printError(err)
	} else {
		printOk("Location added")
	}
}

func locationUpdateUpstreamAction(c *cli.Context) {
	if err := client(c).UpdateLocationUpstream(c.Args().Get(0), c.Args().Get(1), c.Args().Get(2)); err != nil {
		printError(err)
	} else {
		printOk("Location upstream updated")
	}
}

func deleteLocationAction(c *cli.Context) {
	err := client(c).DeleteLocation(c.Args().Get(0), c.Args().Get(1))
	if err != nil {
		printError(err)
	} else {
		printOk("Location deleted")
	}
}
