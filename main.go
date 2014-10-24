package main

import (
	"fmt"
	"os"

	"github.com/mailgun/vulcand/plugin/registry"
	"github.com/mailgun/vulcand/service"
)

func main() {
	if err := service.Run(registry.GetRegistry()); err != nil {
		fmt.Printf("Service exited with error: %s\n", err)
		os.Exit(255)
	} else {
		fmt.Println("Service exited gracefully")
	}
}
