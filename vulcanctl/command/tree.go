package command

import (
	"fmt"
	"io"
	"strings"
)

const (
	vLine   = "\u2502"
	rCross  = "\u251C"
	lCorner = "\u2514"
)

type Tree interface {
	Self() string
	GetChildren() []Tree
}

func printTree(w io.Writer, root Tree, depth int, last bool, offset string) {
	// Print self
	tprint(w, fmt.Sprintf("%s%s%s", offset, getConnector(depth, last), root.Self()))

	// No children, we are done
	children := root.GetChildren()
	if len(children) == 0 {
		return
	}

	// We have children, print connector offset
	tprint(w, getOffset(offset, last))
	// Compute child offset, in case if we are not the last child
	// add vertical line | to connect our parent to the last child
	childOffset := getChildOffset(offset, last)

	for i, c := range children {
		printTree(w, c, depth+1, i == len(children)-1, childOffset)
		if i != len(children)-1 {
			tprint(w, fmt.Sprintf("%s|", childOffset))
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

func tprint(w io.Writer, out string, params ...interface{}) {
	s := fmt.Sprintf(out, params...)
	s = strings.Replace(s, "+-", rCross, -1)
	s = strings.Replace(s, "|", vLine, -1)
	fmt.Fprintf(w, "%s\n", s)
}

type StringTree struct {
	Node     string
	Children []*StringTree
}

func (s *StringTree) AddChild(c *StringTree) {
	s.Children = append(s.Children, c)
}

func (s *StringTree) Self() string {
	return s.Node
}

func (s *StringTree) GetChildren() []Tree {
	out := make([]Tree, len(s.Children))
	for i, _ := range out {
		out[i] = s.Children[i]
	}
	return out
}
