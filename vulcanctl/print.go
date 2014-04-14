package main

import (
	"fmt"
	. "github.com/mailgun/vulcand/backend"
	"strings"
)

const (
	vLine   = "\u2502"
	rCross  = "\u251C"
	lCorner = "\u2514"
)

type Tree interface {
	Self() string
	Children() []Tree
}

func printTree(root Tree, depth int, last bool, offset string) {
	// Print self
	tprint(fmt.Sprintf("%s%s%s", offset, getConnector(depth, last), root.Self()))

	// No children, we are done
	children := root.Children()
	if len(children) == 0 {
		return
	}

	// We have children, print connector offset
	tprint(getOffset(offset, last))
	// Compute child offset, in case if we are not the last child
	// add vertical line | to connect our parent to the last child
	childOffset := getChildOffset(offset, last)

	for i, c := range children {
		printTree(c, depth+1, i == len(children)-1, childOffset)
		if i != len(children)-1 {
			tprint(fmt.Sprintf("%s|", childOffset))
		}
	}
}

func getConnector(depth int, last bool) string {
	if depth == 0 {
		return ""
	}
	if last {
		return lCorner
	}
	return rCross
}

func getChildOffset(offset string, last bool) string {
	if last {
		return fmt.Sprintf("%s  ", offset)
	}
	// in case if we are not the last child
	// add vertical line | to connect our parent to the last child
	return fmt.Sprintf("%s| ", offset)
}

func getOffset(offset string, last bool) string {
	if last {
		return fmt.Sprintf("%s  |", offset)
	}
	return fmt.Sprintf("%s| |", offset)
}

type VulcanTree struct {
	root interface{}
}

func (vt *VulcanTree) Self() string {
	switch r := (vt.root).(type) {
	case []*Host:
		return "[hosts]"
	case []*Upstream:
		return "[upstreams]"
	case *Host:
		return r.String()
	case *Location:
		return r.String()
	case *Upstream:
		return r.String()
	case *Endpoint:
		return r.String()
	}
	return "unknown"
}

func (vt *VulcanTree) Children() []Tree {
	switch r := (vt.root).(type) {
	case []*Host:
		return hostsToTrees(r)
	case []*Upstream:
		return upstreamsToTrees(r)
	case *Host:
		return locationsToTrees(r.Locations)
	case *Upstream:
		return endpointsToTrees(r.Endpoints)
	case *Location:
		return upstreamsToTrees([]*Upstream{r.Upstream})
	}
	return nil
}

func hostsToTrees(in []*Host) []Tree {
	out := make([]Tree, len(in))
	for i, _ := range out {
		out[i] = &VulcanTree{root: in[i]}
	}
	return out
}

func locationsToTrees(in []*Location) []Tree {
	out := make([]Tree, len(in))
	for i, _ := range out {
		out[i] = &VulcanTree{root: in[i]}
	}
	return out
}

func upstreamsToTrees(in []*Upstream) []Tree {
	out := make([]Tree, len(in))
	for i, _ := range out {
		out[i] = &VulcanTree{root: in[i]}
	}
	return out
}

func endpointsToTrees(in []*Endpoint) []Tree {
	out := make([]Tree, len(in))
	for i, _ := range out {
		out[i] = &VulcanTree{root: in[i]}
	}
	return out
}

func printStatus(response *StatusResponse, err error) {
	if err != nil {
		printError(err)
	} else {
		printOk(response.Message)
	}
}

func printError(err error) {
	fmt.Printf("[ERROR]: %s\n", err)
}

func printOk(message string) {
	fmt.Printf("[SUCCESS]: %s\n", message)
}

func printHosts(hosts []*Host) {
	tprint("")
	printTree(&VulcanTree{root: hosts}, 0, true, "")
}

func printUpstreams(upstreams []*Upstream) {
	tprint("")
	printTree(&VulcanTree{root: upstreams}, 0, true, "")
}

func tprint(out string, params ...interface{}) {
	s := fmt.Sprintf(out, params...)
	s = strings.Replace(s, "+-", rCross, -1)
	s = strings.Replace(s, "|", vLine, -1)
	fmt.Println(s)
}
