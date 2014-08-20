package main

import (
	"os"

	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcand/plugin/registry"
	"github.com/mailgun/vulcand/vulcanctl/command"
)

var vulcanUrl string

func main() {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	cmd := command.NewCommand(registry.GetRegistry())
	err := cmd.Run(os.Args)
	if err != nil {
		log.Errorf("Error: %s\n", err)
	}
}
