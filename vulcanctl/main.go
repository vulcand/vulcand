package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	. "github.com/mailgun/vulcand/proxy"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "vulcanctl"
	app.Usage = "Command line interface to a running vulcan instance"
	app.Flags = []cli.Flag{
		cli.StringFlag{"vulcan", "http://localhost:8182", "Url for vulcan server"},
	}
	app.Commands = []cli.Command{
		{
			Name:      "servers",
			ShortName: "s",
			Usage:     "Show vulcan servers",
			Flags:     app.Flags,
			Action: func(c *cli.Context) {
				out, err := client(c).GetServers()
				if err != nil {
					printError(err)
				} else {
					printServers(out)
				}
			},
		},
	}
	app.Run(os.Args)
}

func client(c *cli.Context) *Client {
	return NewClient(c.String("vulcan"))
}

func printError(err error) {
	fmt.Printf("ERROR: %s", err)
}

func printServers(servers []Server) {
	fmt.Printf("vulcan\n")
	fmt.Printf("|\n")
	for _, s := range servers {
		fmt.Printf("+- server(host=%s)\n", s.Name)
		for _, l := range s.Locations {
			fmt.Printf("   |\n")
			fmt.Printf("   +- location(path=%s)\n", l.Path)
			for _, e := range l.Upstream.Endpoints {
				fmt.Printf("      |\n")
				fmt.Printf("      +- endpoint(url=%s)\n", e.Url)
			}
		}
	}
}
