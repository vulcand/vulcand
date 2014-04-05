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
		NewHostCommand(),
		NewLocationCommand(),
		NewUpstreamCommand(),
		NewListUpstreamsCommand(),
		NewEndpointCommand(),
	}
	app.Run(os.Args)
}

func flags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{"vulcan", "http://localhost:8182", "Url for vulcan server"},
	}
}

func client(c *cli.Context) *Client {
	return NewClient(c.String("vulcan"))
}
