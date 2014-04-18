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
	addFlags := []cli.Flag{
		cli.StringFlag{"id", "", "rate id"},
		cli.StringFlag{"host", "", "location's host"},
		cli.StringFlag{"loc", "", "location"},
		cli.StringFlag{"var", "client.ip", "variable to rate against, e.g. client.ip, request.host or request.header.X-Header"},
		cli.IntFlag{"requests", 1, "amount of requests"},
		cli.IntFlag{"period", 1, "rate limit period in seconds"},
		cli.IntFlag{"burst", 1, "allowed burst"},
	}
	return []cli.Command{
		{
			Name:   "add",
			Usage:  "Add a new rate to a location",
			Flags:  addFlags,
			Action: addRateLimitAction,
		},
		{
			Name:   "update",
			Usage:  "Update existing rate",
			Flags:  addFlags,
			Action: updateRateLimitAction,
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
			c.String("requests"),
			c.String("period"),
			c.String("burst")))
}

func updateRateLimitAction(c *cli.Context) {
	printStatus(
		client(c).UpdateRateLimit(
			c.String("host"),
			c.String("loc"),
			c.String("id"),
			c.String("var"),
			c.String("requests"),
			c.String("period"),
			c.String("burst")))
}

func deleteRateLimitAction(c *cli.Context) {
	printStatus(client(c).DeleteRateLimit(
		c.String("host"),
		c.String("loc"),
		c.String("id")))
}
