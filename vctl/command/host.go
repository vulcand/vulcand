package command

import (
	"fmt"
	"time"

	"github.com/codegangsta/cli"
	"github.com/vulcand/vulcand/engine"
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
				Name: "upsert",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name", Usage: "hostname"},
					cli.StringFlag{Name: "privateKey", Usage: "Path to a private key"},
					cli.StringFlag{Name: "cert", Usage: "Path to a certificate"},

					cli.BoolFlag{Name: "ocsp", Usage: "Turn OCSP on"},
					cli.BoolFlag{Name: "ocspSkipCheck", Usage: "Insecure: skip signature checking for the OCSP certificate"},
					cli.DurationFlag{Name: "ocspPeriod", Usage: "optional OCSP period", Value: time.Hour},
					cli.StringSliceFlag{Name: "ocspResponder", Usage: "Optional list of OCSP responders", Value: &cli.StringSlice{}},
				},
				Usage:  "Update or insert a new host to vulcan proxy",
				Action: cmd.upsertHostAction,
			},
			{
				Name: "rm",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name", Usage: "hostname"},
				},
				Usage:  "Remove a host from vulcan",
				Action: cmd.deleteHostAction,
			},
		},
	}
}

func (cmd *Command) printHostsAction(c *cli.Context) error {
	hosts, err := cmd.client.GetHosts()
	if err != nil {
		return err
	}
	cmd.printHosts(hosts)
	return nil
}

func (cmd *Command) printHostAction(c *cli.Context) error {
	host, err := cmd.client.GetHost(engine.HostKey{Name: c.String("name")})
	if err != nil {
		return err
	}
	cmd.printHost(host)
	return nil
}

func (cmd *Command) upsertHostAction(c *cli.Context) error {
	host, err := engine.NewHost(c.String("name"), engine.HostSettings{})
	if err != nil {
		return err
	}
	if c.String("cert") != "" || c.String("privateKey") != "" {
		keyPair, err := readKeyPair(c.String("cert"), c.String("privateKey"))
		if err != nil {
			return fmt.Errorf("failed to read key pair: %s", err)
		}
		host.Settings.KeyPair = keyPair
	}
	host.Settings.OCSP = engine.OCSPSettings{
		Enabled:            c.Bool("ocsp"),
		SkipSignatureCheck: c.Bool("ocspSkipCheck"),
		Period:             c.Duration("ocspPeriod").String(),
		Responders:         c.StringSlice("ocspResponder"),
	}
	if err := cmd.client.UpsertHost(*host); err != nil {
		return err
	}
	cmd.printOk("host added")
	return nil
}

func (cmd *Command) deleteHostAction(c *cli.Context) error {
	if err := cmd.client.DeleteHost(engine.HostKey{Name: c.String("name")}); err != nil {
		return err
	}
	cmd.printOk("host deleted")
	return nil
}
