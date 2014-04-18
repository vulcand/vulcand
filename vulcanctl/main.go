package main

import (
	"github.com/mailgun/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "vulcanctl"
	app.Usage = "Command line interface to a running vulcan instance"
	app.Flags = flags()

	app.Commands = []cli.Command{
		NewStatusCommand(),
	}

	location := app.AddSubcommand(NewLocationCommand())
	location.Commands = NewLocationSubcommands()

	host := app.AddSubcommand(NewHostCommand())
	host.Commands = NewHostSubcommands()

	endpoint := app.AddSubcommand(NewEndpointCommand())
	endpoint.Commands = NewEndpointSubcommands()

	upstream := app.AddSubcommand(NewUpstreamCommand())
	upstream.Commands = NewUpstreamSubcommands()

	ratelimit := app.AddSubcommand(NewRateLimitCommand())
	ratelimit.Commands = NewRateLimitSubcommands()

	connlimit := app.AddSubcommand(NewConnLimitCommand())
	connlimit.Commands = NewConnLimitSubcommands()

	app.Run(os.Args)
}

func flags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{"vulcan", "http://localhost:8182", "Url for vulcan server"},
	}
}

func client(c *cli.Context) *Client {
	vulcanUrl := c.String("vulcan")
	if len(vulcanUrl) == 0 {
		vulcanUrl = "http://localhost:8182"
	}
	return NewClient(vulcanUrl)
}
