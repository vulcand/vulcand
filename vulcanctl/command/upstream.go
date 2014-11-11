package command

import (
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/backend"
)

func NewUpstreamCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "upstream",
		Usage: "Operations with vulcan upstreams",
		Subcommands: []cli.Command{
			{
				Name:   "add",
				Usage:  "Add a new upstream to vulcan",
				Action: cmd.addUpstreamAction,
				Flags: append([]cli.Flag{
					cli.StringFlag{Name: "id", Usage: "upstream id"}},
					upstreamOptions()...),
			},
			{
				Name:   "rm",
				Usage:  "Remove upstream from vulcan",
				Action: cmd.deleteUpstreamAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "upstream id"},
				},
			},
			{
				Name:   "ls",
				Usage:  "List upstreams",
				Action: cmd.listUpstreamsAction,
			},
			{
				Name:   "set_options",
				Usage:  "Update upstream options",
				Action: cmd.upstreamUpdateOptionsAction,
				Flags: append([]cli.Flag{
					cli.StringFlag{Name: "id", Usage: "upstream id"}},
					upstreamOptions()...),
			},
		},
	}
}

func (cmd *Command) addUpstreamAction(c *cli.Context) {
	options, err := getUpstreamOptions(c)
	if err != nil {
		cmd.printError(err)
		return
	}

	u, err := cmd.client.AddUpstreamWithOptions(c.String("id"), options)
	cmd.printResult("%s added", u, err)
}

func (cmd *Command) deleteUpstreamAction(c *cli.Context) {
	cmd.printStatus(cmd.client.DeleteUpstream(c.String("id")))
}

func (cmd *Command) listUpstreamsAction(c *cli.Context) {
	out, err := cmd.client.GetUpstreams()
	if err != nil {
		cmd.printError(err)
	} else {
		cmd.printUpstreams(out)
	}
}

func (cmd *Command) upstreamUpdateOptionsAction(c *cli.Context) {
	options, err := getUpstreamOptions(c)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printStatus(cmd.client.UpdateUpstreamOptions(c.String("id"), options))
}

func getUpstreamOptions(c *cli.Context) (backend.UpstreamOptions, error) {
	o := backend.UpstreamOptions{}

	o.Timeouts.Read = c.Duration("readTimeout").String()
	o.Timeouts.Dial = c.Duration("dialTimeout").String()
	o.Timeouts.TlsHandshake = c.Duration("handshakeTimeout").String()

	o.KeepAlive.Period = c.Duration("keepAlivePeriod").String()
	o.KeepAlive.MaxIdleConnsPerHost = c.Int("maxIdleConns")

	return o, nil
}

func upstreamOptions() []cli.Flag {
	return []cli.Flag{
		// Timeouts
		cli.DurationFlag{Name: "readTimeout", Usage: "read timeout"},
		cli.DurationFlag{Name: "dialTimeout", Usage: "dial timeout"},
		cli.DurationFlag{Name: "handshakeTimeout", Usage: "TLS handshake timeout"},

		// Keep-alive parameters
		cli.StringFlag{Name: "keepAlivePeriod", Usage: "keep-alive period"},
		cli.IntFlag{Name: "maxIdleConns", Usage: "maximum idle connections per host"},
	}
}
