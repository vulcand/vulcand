package main

import (
	"github.com/mailgun/cli"
)

func NewRateLimitCommand() cli.Command {
	return cli.Command{
		Name:  "ratelimit",
		Usage: "Operations with ratelimits",
	}
}

func NewRateLimitSubcommands() []cli.Command {
	return []cli.Command{
		{
			Name:  "add",
			Usage: "Add a new rate to a location",
			Flags: []cli.Flag{
				cli.StringFlag{"id", "", "rate id"},
				cli.StringFlag{"host", "", "location's host"},
				cli.StringFlag{"loc", "", "location"},
				cli.StringFlag{"var", "client.ip", "variable to rate against, e.g. client.ip, request.host or request.header.X-Header"},
				cli.IntFlag{"reqs", -1, "amount of requests"},
				cli.IntFlag{"period", 1, "rate limit period in seconds"},
				cli.IntFlag{"burst", 1, "allowed burst"},
			},
			Action: addRateLimitAction,
		},
		{
			Name:  "rm",
			Usage: "Delete rate from location",
			Flags: []cli.Flag{
				cli.StringFlag{"id", "", "rate id"},
				cli.StringFlag{"host", "", "location's host"},
				cli.StringFlag{"loc", "", "location"},
			},
			Action: deleteRateLimitAction,
		},
	}
}

func addRateLimitAction(c *cli.Context) {
	printStatus(
		client(c).AddRateLimit(
			c.String("host"),
			c.String("loc"),
			c.String("id"),
			c.String("var"),
			c.String("reqs"),
			c.String("period"),
			c.String("burst")))
}

func deleteRateLimitAction(c *cli.Context) {
	printStatus(client(c).DeleteRateLimit(
		c.String("host"),
		c.String("loc"),
		c.String("id")))
}
