package command

import (
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	. "github.com/mailgun/vulcand/backend"
	. "github.com/mailgun/vulcand/plugin"
)

func NewMiddlewareCommands(cmd *Command) []cli.Command {
	out := []cli.Command{}
	for _, spec := range cmd.registry.GetSpecs() {
		if spec.CliFlags != nil && spec.FromCli != nil {
			out = append(out, makeMiddlewareCommands(cmd, spec))
		}
	}
	return out
}

func makeMiddlewareCommands(cmd *Command, spec *MiddlewareSpec) cli.Command {
	flags := append([]cli.Flag{}, spec.CliFlags...)
	flags = append(flags,
		cli.StringFlag{Name: "host", Usage: "location host"},
		cli.StringFlag{Name: "location, loc", Usage: "location id"},
		cli.IntFlag{Name: "priority", Value: 1, Usage: "middleware priority, smaller values are lower"},
		cli.StringFlag{Name: "id", Usage: fmt.Sprintf("%s id", spec.Type)})

	return cli.Command{
		Name:  spec.Type,
		Usage: fmt.Sprintf("Operations on %s middlewares", spec.Type),
		Subcommands: []cli.Command{
			{
				Name:   "add",
				Usage:  fmt.Sprintf("Add a new %s to location", spec.Type),
				Flags:  flags,
				Action: makeAddMiddlewareAction(cmd, spec),
			},
			{
				Name:   "update",
				Usage:  fmt.Sprintf("Update %s", spec.Type),
				Action: makeUpdateMiddlewareAction(cmd, spec),
				Flags:  flags,
			},
			{
				Name:   "rm",
				Usage:  fmt.Sprintf("Remove %s from location", spec.Type),
				Action: makeDeleteMiddlewareAction(cmd, spec),
				Flags: []cli.Flag{
					cli.StringFlag{Name: "host", Usage: "location's host"},
					cli.StringFlag{Name: "location, loc", Usage: "Location id"},
					cli.StringFlag{Name: "id", Usage: fmt.Sprintf("%s id", spec.Type)},
				},
			},
		},
	}
}

func makeAddMiddlewareAction(cmd *Command, spec *MiddlewareSpec) func(c *cli.Context) {
	return func(c *cli.Context) {
		m, err := spec.FromCli(c)
		if err != nil {
			cmd.printError(err)
		} else {
			mi := &MiddlewareInstance{Id: c.String("id"), Middleware: m, Type: spec.Type, Priority: c.Int("priority")}
			response, err := cmd.client.AddMiddleware(spec, c.String("host"), c.String("loc"), mi)
			cmd.printResult("%s added", response, err)
		}
	}
}

func makeUpdateMiddlewareAction(cmd *Command, spec *MiddlewareSpec) func(c *cli.Context) {
	return func(c *cli.Context) {
		m, err := spec.FromCli(c)
		if err != nil {
			cmd.printError(err)
		} else {
			mi := &MiddlewareInstance{Id: c.String("id"), Middleware: m, Type: spec.Type}
			response, err := cmd.client.UpdateMiddleware(spec, c.String("host"), c.String("loc"), mi)
			cmd.printResult("%s updated", response, err)
		}
	}
}

func makeDeleteMiddlewareAction(cmd *Command, spec *MiddlewareSpec) func(c *cli.Context) {
	return func(c *cli.Context) {
		cmd.printStatus(cmd.client.DeleteMiddleware(spec, c.String("host"), c.String("loc"), c.String("id")))
	}
}
