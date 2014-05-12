package main

import (
	"github.com/codegangsta/cli"
)

func NewStatusCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:      "status",
		ShortName: "s",
		Usage:     "Show vulcan status and configuration",
		Action:    cmd.statusAction,
	}
}

func (cmd *Command) statusAction(c *cli.Context) {
	out, err := cmd.client.GetHosts()
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printHosts(out)
	}
}
