package main

import (
	"github.com/codegangsta/cli"
)

func NewLocationCommand() cli.Command {
	return NewGroupCommand(GroupCommand{
		Name:  "location",
		Usage: "Operations with vulcan locations",
		Flags: flags(),
		Subcommands: []cli.Command{
			{
				Name:   "add",
				Flags:  flags(),
				Usage:  "Add a new location to vulcan proxy",
				Action: addLocationAction,
			},
			{
				Name:   "rm",
				Flags:  flags(),
				Usage:  "Remove a location from vulcan",
				Action: deleteLocationAction,
			},
		},
	})
}

func addLocationAction(c *cli.Context) {
	if err := client(c).AddLocation(c.Args().Get(0), c.Args().Get(1), c.Args().Get(2), c.Args().Get(3)); err != nil {
		printError(err)
	} else {
		printOk("Location added")
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
