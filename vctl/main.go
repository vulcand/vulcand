package main

import (
	"os"

	"github.com/vulcand/vulcand/plugin/registry"
	"github.com/vulcand/vulcand/vctl/command"
)

var vulcanUrl string

func main() {
	cmd := command.NewCommand(registry.GetRegistry())
	err := cmd.Run(os.Args)
	if err != nil {
		cmd.PrintError(err)
	}
}
