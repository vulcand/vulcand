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
				Name:   "ls",
				Usage:  "List all hosts",
				Flags:  []cli.Flag{},
				Action: cmd.printHostsAction,
			},
			{
				Name:  "show",
				Usage: "Show host details",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name", Usage: "hostname"},
				},
				Action: cmd.printHostAction,
			},
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
				Name: "set_keypair",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name", Usage: "hostname"},
					cli.StringFlag{Name: "privateKey", Usage: "Path to a private key"},
					cli.StringFlag{Name: "cert", Usage: "Path to a certificate"},
				},
				Usage:  "Set host key pair",
				Action: cmd.updateHostKeyPairAction,
			},
		},
	}
}

func (cmd *Command) printHostsAction(c *cli.Context) {
	hosts, err := cmd.client.GetHosts()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printHosts(hosts)
}

func (cmd *Command) printHostAction(c *cli.Context) {
	host, err := cmd.client.GetHost(c.String("name"))
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printHost(host)
}

func (cmd *Command) addHostAction(c *cli.Context) {
	host, err := cmd.client.AddHost(c.String("name"))
	cmd.printResult("%s added", host, err)
}

func (cmd *Command) updateHostKeyPairAction(c *cli.Context) {
	keyPair, err := readKeyPair(c.String("cert"), c.String("privateKey"))
	if err != nil {
		cmd.printError(fmt.Errorf("failed to read key pair: %s", err))
		return
	}

	host, err := cmd.client.UpdateHostKeyPair(c.String("name"), keyPair)
	cmd.printResult("%s key pair updated", host, err)
}

func (cmd *Command) deleteHostAction(c *cli.Context) {
	cmd.printStatus(cmd.client.DeleteHost(c.String("name")))
}
