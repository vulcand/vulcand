package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/mailgun/vulcand/plugin/registry"
	"github.com/mailgun/vulcand/service"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	runtime.GOMAXPROCS(runtime.NumCPU())
	if err := service.Run(registry.GetRegistry()); err != nil {
		fmt.Printf("Service exited with error: %s\n", err)
		os.Exit(255)
	} else {
		fmt.Println("Service exited gracefully")
	}
}
