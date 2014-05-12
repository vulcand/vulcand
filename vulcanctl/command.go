package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/api"
	"io"
	"os"
	"strings"
)

type Command struct {
	vulcanUrl string
	client    *api.Client
	out       io.Writer
}

func NewCommand() *Command {
	return &Command{
		out: os.Stdout,
	}
}

func (cmd *Command) Run(args []string) error {
	url, args, err := findVulcanUrl(args)
	if err != nil {
		return err
	}
	cmd.vulcanUrl = url
	cmd.client = api.NewClient(cmd.vulcanUrl)

	app := cli.NewApp()
	app.Name = "vulcanctl"
	app.Usage = "Command line interface to a running vulcan instance"
	app.Flags = flags()

	app.Commands = []cli.Command{
		NewStatusCommand(cmd),
		NewHostCommand(cmd),
		NewUpstreamCommand(cmd),
		NewLocationCommand(cmd),
		NewEndpointCommand(cmd),
	}
	app.Commands = append(app.Commands, NewMiddlewareCommands(cmd)...)
	return app.Run(args)
}

// This function extracts vulcan url from the command line regardless of it's position
// this is a workaround, as cli libary does not support "superglobal" urls yet.
func findVulcanUrl(args []string) (string, []string, error) {
	for i, arg := range args {
		if strings.HasPrefix(arg, "--vulcan=") || strings.HasPrefix(arg, "-vulcan=") {
			out := strings.Split(arg, "=")
			return out[1], cut(i, i+1, args), nil
		} else if strings.HasPrefix(arg, "-vulcan") || strings.HasPrefix(arg, "--vulcan") {
			// This argument should not be the last one
			if i > len(args)-2 {
				return "", nil, fmt.Errorf("Provide a valid vulcan URL")
			}
			return args[i+1], cut(i, i+2, args), nil
		}
	}
	return "http://localhost:8182", args, nil
}

func cut(i, j int, args []string) []string {
	s := []string{}
	s = append(s, args[:i]...)
	return append(s, args[j:]...)
}

func flags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{"vulcan", "http://localhost:8182", "Url for vulcan server"},
	}
}
