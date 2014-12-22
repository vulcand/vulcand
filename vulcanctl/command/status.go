package command

import (
	"fmt"
	"time"

	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/BTBurke/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcand/backend"
)

func NewTopCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "top",
		Usage: "Show vulcan status and configuration in top-style mode",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "limit", Usage: "How many top entries to show", Value: 20},
			cli.IntFlag{Name: "refresh", Usage: "How often refresh (in seconds), if 0 - will display only once", Value: 1},
			cli.StringFlag{Name: "upstream, up", Usage: "Filter locations and endpoints by upstream id", Value: ""},
			cli.StringFlag{Name: "host", Usage: "Filter locations by hostname", Value: ""},
		},
		Action: cmd.topAction,
	}
}

func (cmd *Command) topAction(c *cli.Context) {
	cmd.overviewAction(c.String("hostname"), c.String("upstream"), c.Int("refresh"), c.Int("limit"))
}

func (cmd *Command) overviewAction(hostname, upstreamId string, watch int, limit int) {
	for {
		locations, err := cmd.client.GetTopLocations(hostname, upstreamId, limit)
		if err != nil {
			cmd.printError(err)
			locations = []*backend.Location{}
		}

		endpoints, err := cmd.client.GetTopEndpoints(upstreamId, limit)
		if err != nil {
			cmd.printError(err)
			endpoints = []*backend.Endpoint{}
		}
		t := time.Now()
		if watch != 0 {
			goterm.Clear()
			goterm.MoveCursor(1, 1)
			goterm.Flush()
			fmt.Fprintf(cmd.out, "%s Every %d seconds. Top %d entries\n\n", t.Format("2006-01-02 15:04:05"), watch, limit)
		}

		cmd.printOverview(locations, endpoints)
		if watch != 0 {
			goterm.Flush()
		} else {
			return
		}
		time.Sleep(time.Second * time.Duration(watch))
	}
}
