package command

import (
	"fmt"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
)

func NewStatusCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "status",
		Usage: "Show vulcan status and configuration",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "limit", Usage: "How many top locations to show", Value: 20},
			cli.IntFlag{Name: "watch, w", Usage: "Watch and refresh every given amount of seconds", Value: 0},
		},
		Action: cmd.statusAction,
	}
}

func NewTopCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "top",
		Usage: "Show vulcan status and configuration in top-style mode",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "limit", Usage: "How many top entries to show", Value: 20},
			cli.StringFlag{Name: "type", Usage: "Top type (location, upstream)", Value: "location"},
		},
		Action: cmd.topAction,
	}
}

func (cmd *Command) topAction(c *cli.Context) {
	cmd.overviewAction(1, c.Int("limit"))
}

func (cmd *Command) statusAction(c *cli.Context) {
	cmd.overviewAction(c.Int("watch"), c.Int("limit"))
}

func (cmd *Command) overviewAction(watch int, limit int) {
	for {
		hosts, err := cmd.client.GetHosts()
		if err != nil {
			cmd.printError(err)
			return
		}

		upstreams, err := cmd.client.GetUpstreams()
		if err != nil {
			cmd.printError(err)
			return
		}

		goterm.Clear()
		goterm.MoveCursor(1, 1)
		goterm.Flush()
		t := time.Now()
		fmt.Fprintf(cmd.out, "%s Every %d seconds. Top %d entries\n\n", t.Format("2006-01-02 15:04:05"), watch, limit)
		cmd.printOverview(hosts, upstreams, limit)
		goterm.Flush()
		if watch == 0 {
			return
		}
		time.Sleep(time.Second * time.Duration(watch))
	}
}
