package command

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"strings"
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

func (cmd *Command) updateLogSeverityAction(c *cli.Context) error {
	sev, err := log.ParseLevel(strings.ToLower(c.String("severity")))
	if err != nil {
		return err
	}
	if err := cmd.client.UpdateLogSeverity(sev); err != nil {
		return err
	}
	cmd.printOk("log severity updated")
	return nil
}

func (cmd *Command) getLogSeverityAction(c *cli.Context) error {
	sev, err := cmd.client.GetLogSeverity()
	if err != nil {
		return err
	}
	cmd.printOk("severity: %v", sev)
	return nil
}
