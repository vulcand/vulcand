package main

import (
	"fmt"
	"github.com/mailgun/vulcand/service"
	"os"
)

func main() {
	r, err := GetRegistry()
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
