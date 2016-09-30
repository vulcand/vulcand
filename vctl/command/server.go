package command

import (
	"github.com/codegangsta/cli"
	"github.com/vulcand/vulcand/engine"
)

func NewServerCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "server",
		Usage: "Operations with servers",
		Subcommands: []cli.Command{
			{
				Name:  "ls",
				Usage: "List all servers for a given backend",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "backend, b", Usage: "backend id"},
				},
				Action: cmd.printServersAction,
			},
			{
				Name:  "show",
				Usage: "Show server details",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "server id"},
					cli.StringFlag{Name: "backend, b", Usage: "backend id"},
				},
				Action: cmd.printServerAction,
			},
			{
				Name:   "upsert",
				Usage:  "Add a new endpoint to location",
				Action: cmd.upsertServerAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "server id"},
					cli.StringFlag{Name: "backend, b", Usage: "backend id"},
					cli.StringFlag{Name: "url", Usage: "url in form <scheme>://<host>:<port>"},
					cli.DurationFlag{Name: "ttl", Usage: "ttl"},
				},
			},
			{
				Name:  "rm",
				Usage: "Remove endpoint from location",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "id", Usage: "endpoint id"},
					cli.StringFlag{Name: "backend, b", Usage: "backend id"},
				},
				Action: cmd.deleteServerAction,
			},
		},
	}
}

func (cmd *Command) upsertServerAction(c *cli.Context) error {
	s, err := engine.NewServer(c.String("id"), c.String("url"))
	if err != nil {
		return err
	}
	if err := cmd.client.UpsertServer(engine.BackendKey{Id: c.String("backend")}, *s, c.Duration("ttl")); err != nil {
		return err
	}
	cmd.printOk("server upserted")
	return nil
}

func (cmd *Command) deleteServerAction(c *cli.Context) error {
	sk := engine.ServerKey{BackendKey: engine.BackendKey{Id: c.String("backend")}, Id: c.String("id")}
	if err := cmd.client.DeleteServer(sk); err != nil {
		return err
	}
	cmd.printOk("Server %v deleted", sk.Id)
	return nil
}

func (cmd *Command) printServersAction(c *cli.Context) error {
	srvs, err := cmd.client.GetServers(engine.BackendKey{Id: c.String("backend")})
	if err != nil {
		return err
	}
	cmd.printServers(srvs)
	return nil
}

func (cmd *Command) printServerAction(c *cli.Context) error {
	s, err := cmd.client.GetServer(engine.ServerKey{Id: c.String("id"), BackendKey: engine.BackendKey{Id: c.String("backend")}})
	if err != nil {
		return err
	}
	cmd.printServer(s)
	return nil
}
