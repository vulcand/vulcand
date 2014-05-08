package main

import (
	"github.com/codegangsta/cli"
)

func NewStatusCommand() cli.Command {
	return cli.Command{
		Name:      "status",
		ShortName: "s",
		Usage:     "Show vulcan status and configuration",
		Action:    StatusAction,
	}
}

func StatusAction(c *cli.Context) {
	out, err := client(c).GetHosts()
	if err != nil {
		printError(err)
	} else {
		printHosts(out)
	}
}
