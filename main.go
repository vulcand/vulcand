package main

import (
	"fmt"
	"github.com/mailgun/vulcand/service"
	"os"
)

func main() {
	options, err := service.ParseCommandLine()
	if err != nil {
		fmt.Printf("Failed to parse command line: %s\n", err)
		return
	}
	service := service.NewService(options)
	if err := service.Start(); err != nil {
		fmt.Printf("Service exited with error: %s\n", err)
		os.Exit(255)
	} else {
		fmt.Println("Service exited gracefully")
	}
}
