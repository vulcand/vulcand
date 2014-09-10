package command

import (
	"fmt"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
)

func NewHostCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "host",
		Usage: "Operations with vulcan hosts",
		Subcommands: []cli.Command{
			{
				Name: "add",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name", Usage: "hostname"},
				},
				Usage:  "Add a new host to vulcan proxy",
				Action: cmd.addHostAction,
			},
			{
				Name: "rm",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name", Usage: "hostname"},
				},
				Usage:  "Remove a host from vulcan",
				Action: cmd.deleteHostAction,
			},
			{
				Name: "set_cert",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name", Usage: "hostname"},
					cli.StringFlag{Name: "private", Usage: "Path to a private key"},
					cli.StringFlag{Name: "public", Usage: "Path to a public key"},
				},
				Usage:  "Set host certificate",
				Action: cmd.updateHostCertAction,
			},
		},
	}
}

func (cmd *Command) addHostAction(c *cli.Context) {
	host, err := cmd.client.AddHost(c.String("name"))
	cmd.printResult("%s added", host, err)
}

func (cmd *Command) updateHostCertAction(c *cli.Context) {
	cert, err := readCert(c.String("private"), c.String("public"))
	if err != nil {
		cmd.printError(fmt.Errorf("Failed to read certificate: %s", err))
		return
	}

	host, err := cmd.client.UpdateHostCert(c.String("name"), cert)
	cmd.printResult("%s certificate updated", host, err)
}

func (cmd *Command) deleteHostAction(c *cli.Context) {
	cmd.printStatus(cmd.client.DeleteHost(c.String("name")))
}
