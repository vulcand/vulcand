package main

const mainTemplate = `package main

import (
	"fmt"
	"github.com/mailgun/vulcand/service"
	"{{.PackagePath}}/registry"
	"os"
)

func main() {
	r, err := registry.GetRegistry()
	if err != nil {
		fmt.Printf("Service exited with error: %s\n", err)
		os.Exit(255)
	}
	if err := service.Run(r); err != nil {
		fmt.Printf("Service exited with error: %s\n", err)
		os.Exit(255)
	} else {
		fmt.Println("Service exited gracefully")
	}
}
`

const registryTemplate = `package registry

import (
	"github.com/mailgun/vulcand/plugin"
	{{range .Packages}}
	"{{.}}"
	{{end}}
)

func GetRegistry() (*plugin.Registry, error) {
	r := plugin.NewRegistry()

	specs := []*plugin.MiddlewareSpec{
		{{range .Packages}}
		{{.Name}}.GetSpec(),
       {{end}}
	}

	for _, spec := range specs {
		if err := r.AddSpec(spec); err != nil {
			return nil, err
		}
	}
	return r, nil
}
`

const vulcanctlTemplate = `package main

import (
    "github.com/mailgun/log"
	"github.com/mailgun/vulcand/vctl/command"
	"{{.PackagePath}}/registry"
	"os"
)

var vulcanUrl string

func main() {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

    r, err := registry.GetRegistry()
	if err != nil {
		log.Errorf("Error: %s\n", err)
        return
	}
	cmd := command.NewCommand(r)
	if err := cmd.Run(os.Args); err != nil {
		log.Errorf("Error: %s\n", err)
	}
}
`
