package main

import (
	"fmt"
	"github.com/codegangsta/cli"
)

type GroupCommand struct {
	Name        string
	Usage       string
	Flags       []cli.Flag
	Subcommands []cli.Command
}

func NewGroupCommand(g GroupCommand) cli.Command {
	return cli.Command{
		Name:  g.Name,
		Usage: g.Usage,
		Flags: g.Flags,
		Action: func(c *cli.Context) {
			app := cli.NewApp()
			app.Name = fmt.Sprintf("%s %s", c.App.Name, g.Name)
			app.Usage = g.Usage
			app.Commands = g.Subcommands
			app.Run(append([]string{app.Name}, c.Args()...))
		},
	}
}
