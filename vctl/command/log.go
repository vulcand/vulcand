package command

import (
	"strings"
	"github.com/vulcand/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	log "github.com/vulcand/vulcand/Godeps/_workspace/src/github.com/Sirupsen/logrus"
)

func NewLogCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name: "log",
		Subcommands: []cli.Command{
			{
				ShortName: "set_severity",
				Usage:     "Set logging severity",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "severity, s"},
				},
				Action: cmd.updateLogSeverityAction,
			},
			{
				ShortName: "get_severity",
				Usage:     "Get logging severity",
				Action:    cmd.getLogSeverityAction,
			},
		},
	}
}

func (cmd *Command) updateLogSeverityAction(c *cli.Context) {
	sev, err := log.ParseLevel(strings.ToLower(c.String("severity")))
	if err != nil {
		cmd.printError(err)
		return
	}
	if err := cmd.client.UpdateLogSeverity(sev); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOk("log severity updated")
}

func (cmd *Command) getLogSeverityAction(c *cli.Context) {
	sev, err := cmd.client.GetLogSeverity()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOk("severity: %v", sev)
}
