package main

import (
	"fmt"
	. "github.com/mailgun/vulcand/backend"
)

func printError(err error) {
	fmt.Printf("ERROR: %s\n", err)
}

func printOk(message string) {
	fmt.Printf("SUCCESS: %s\n", message)
}

func printHosts(hosts []Host) {
	fmt.Printf("vulcan hosts\n")
	fmt.Printf("|\n")
	for _, h := range hosts {
		printHost(h)
	}
}

func printHost(h Host) {
	fmt.Printf("+- host(name=%s)\n", h.Name)
	for _, l := range h.Locations {
		fmt.Printf("   |\n")
		fmt.Printf("   +- location(path=%s, upstream=%s)\n", l.Path, l.Upstream.Name)
		for _, e := range l.Upstream.Endpoints {
			fmt.Printf("      |\n")
			fmt.Printf("      +- endpoint(url=%s)\n", e.Url)
		}
	}
}

func printUpstreams(upstreams []Upstream) {
	fmt.Printf("vulcan upstreams\n")
	fmt.Printf("|\n")
	for _, u := range upstreams {
		fmt.Printf("+- upstream(id=%s)\n", u.Name)
		for _, e := range u.Endpoints {
			fmt.Printf("  |\n")
			fmt.Printf("  +- endpoint(url=%s)\n", e.Url)
		}
	}
}
