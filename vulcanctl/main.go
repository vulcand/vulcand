package main

import (
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"os"
)

var vulcanUrl string

func main() {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	cmd := NewCommand()
	err := cmd.Run(os.Args)
	if err != nil {
		log.Errorf("Error: %s\n", err)
	}
}
