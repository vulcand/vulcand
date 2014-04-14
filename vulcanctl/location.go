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
			Name:  "add",
			Usage: "Add a new location to host",
			Flags: []cli.Flag{
				cli.StringFlag{"id", "", "location id"},
				cli.StringFlag{"host", "", "location's host"},
				cli.StringFlag{"path", "", "location's path (will be matched against request's path)"},
				cli.StringFlag{"up", "", "location's upstream id"},
			},
			Action: addLocationAction,
		},
		{
			Name:   "rm",
			Usage:  "Remove a location from host",
			Action: deleteLocationAction,
			Flags: []cli.Flag{
				cli.StringFlag{"id", "", "location id"},
				cli.StringFlag{"host", "", "location's host"},
			},
		},
		{
			Name:   "set_upstream",
			Usage:  "Update location upstream",
			Action: locationUpdateUpstreamAction,
			Flags: []cli.Flag{
				cli.StringFlag{"id", "", "location id"},
				cli.StringFlag{"host", "", "location's host"},
				cli.StringFlag{"up", "", "location's new upstream"},
			},
		},
	}
}

func addLocationAction(c *cli.Context) {
	printStatus(client(c).AddLocation(c.String("host"), c.String("id"), c.String("path"), c.String("up")))
}

func locationUpdateUpstreamAction(c *cli.Context) {
	printStatus(client(c).UpdateLocationUpstream(c.String("host"), c.String("id"), c.String("up")))
}

func deleteLocationAction(c *cli.Context) {
	printStatus(client(c).DeleteLocation(c.String("host"), c.String("id")))
}
