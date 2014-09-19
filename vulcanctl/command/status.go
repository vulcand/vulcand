package command

import (
	"fmt"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
)

func NewStatusCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:      "status",
		ShortName: "s",
		Usage:     "Show vulcan status and configuration",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "limit", Usage: "How many top locations to show", Value: 20},
			cli.IntFlag{Name: "watch, w", Usage: "Watch and refresh every given amount of seconds", Value: 0},
		},
		Action: cmd.statusAction,
	}
}

func (cmd *Command) statusAction(c *cli.Context) {
	watch := c.Int("watch")
	// One time print and return
	if watch == 0 {
		out, err := cmd.client.GetHosts()
		if err != nil {
			cmd.printError(err)
			return
		}
		cmd.printOverview(out, c.Int("limit"))
		return
	}

	//Loop and get values
	for {
		out, err := cmd.client.GetHosts()
		if err != nil {
			cmd.printError(err)
			return
		}
		// Flush screen
		cmd.out.Write([]byte("\033[2J"))
		cmd.out.Write([]byte("\033[H"))
		// Print overview
		t := time.Now()
		fmt.Fprintf(cmd.out, "%s Every %d seconds. Top %d locations\n\n", t.Format("2006-01-02 15:04:05"), watch, c.Int("limit"))
		cmd.printOverview(out, c.Int("limit"))
		time.Sleep(time.Second * time.Duration(watch))
	}

}
