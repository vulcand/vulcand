package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"os"
	"strings"
)

var vulcanUrl string

func main() {
	url, args, err := findVulcanUrl(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	vulcanUrl = url

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
	app.Run(args)
}

func findVulcanUrl(args []string) (string, []string, error) {

	for i, arg := range args {
		if strings.HasPrefix(arg, "--vulcan=") || strings.HasPrefix(arg, "-vulcan=") {
			out := strings.Split(arg, "=")
			return out[1], cut(i, i+1, args), nil
		} else if strings.HasPrefix(arg, "-vulcan") || strings.HasPrefix(arg, "--vulcan") {
			// This argument should not be the last one
			if i > len(args)-2 {
				return "", nil, fmt.Errorf("Provide a valid vulcan rul")
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

func client(c *cli.Context) *Client {
	return NewClient(vulcanUrl)
}
