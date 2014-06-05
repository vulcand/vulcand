package main

import (
	//	"fmt"
	"github.com/codegangsta/cli"
	log "github.com/mailgun/gotools-log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var vulcanUrl string

func main() {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	app := cli.NewApp()
	app.Name = "vulcanbundle"
	app.Usage = "Command line interface to compile plugins into vulcan binary"
	app.Commands = []cli.Command{
		{
			Name:   "get",
			Usage:  "Get",
			Action: get,
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					"middleware, m",
					&cli.StringSlice{},
					"Path to repo and revision, e.g. github.com/mailgun/vulcand-plugins/auth",
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("Error: %s\n", err)
	}
}

func get(c *cli.Context) {
	b, err := NewBundler(c.StringSlice("middleware"))
	if err != nil {
		log.Errorf("Failed to bundle middlewares: %s", err)
		return
	}
	if err := b.bundle(); err != nil {
		log.Errorf("Failed to bundle middlewares: %s", err)
	}
}

type Bundler struct {
	bundleDir   string
	middlewares []string
}

func NewBundler(middlewares []string) (*Bundler, error) {
	bundleDir, err := filepath.Abs(".")
	if err != nil {
		return nil, err
	}
	return &Bundler{bundleDir: bundleDir, middlewares: middlewares}, nil
}

func (b *Bundler) bundle() error {
	if err := b.getPackages(); err != nil {
		return err
	}
	return nil
}

func (b *Bundler) getPackages() error {
	packages := append([]string{"github.com/mailgun/vulcand"}, b.middlewares...)
	for _, p := range packages {
		log.Infof("Downloading %s into %s", p, b.bundleDir)
		if err := b.runGodep("get", p); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bundler) runGodep(args ...string) error {
	c := exec.Command("godep", args...)
	c.Env = append(envNoGopath(), "GOPATH="+b.bundleDir)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

type Godeps struct {
	ImportPath string
	GoVersion  string
	Packages   []string `json:",omitempty"`
	Deps       []Dependency
}

type Dependency struct {
	ImportPath string
	Comment    string `json:",omitempty"`
	Rev        string
}

func runCommand(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func runGo(gopath string, args ...string) error {
	c := exec.Command("go", args...)
	c.Env = append(envNoGopath(), "GOPATH="+gopath)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func envNoGopath() (a []string) {
	for _, s := range os.Environ() {
		if !strings.HasPrefix(s, "GOPATH=") {
			a = append(a, s)
		}
	}
	return a
}
