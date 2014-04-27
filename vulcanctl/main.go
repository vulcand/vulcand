package main

import (
	"github.com/codegangsta/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "vulcanctl"
	app.Usage = "Command line interface to a running vulcan instance"
	app.Flags = flags()

	app.Commands = []cli.Command{
		NewStatusCommand(),
		NewLocationCommand(),
		NewHostCommand(),
		NewEndpointCommand(),
		NewUpstreamCommand(),
		NewRateLimitCommand(),
		NewConnLimitCommand(),
	}
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
