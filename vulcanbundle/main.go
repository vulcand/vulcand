package main

import (
	"encoding/json"
	"fmt"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"io/ioutil"
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
	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			"middleware, m",
			&cli.StringSlice{},
			"Path to repo and revision, e.g. github.com/mailgun/vulcand-plugins/auth:6653291c990005550b334ce3a516d840a4e040f5",
		},
	}
	app.Action = bundle
	err := app.Run(os.Args)
	if err != nil {
		log.Errorf("Error: %s\n", err)
	}
}

func bundle(c *cli.Context) {
	log.Infof("Compiling middlewares: %s", c.StringSlice("middleware"))
	middlewares, err := parseMiddlewares(c.StringSlice("middleware"))
	if err != nil {
		log.Errorf("Failed to parse middlewares: %s", err)
		return
	}
	if err := bundleMiddlewares(middlewares); err != nil {
		log.Errorf("Failed to bundle middlewares: %s", err)
	}
}

func bundleMiddlewares(middlewares []Dependency) error {
	bundleDir, err := filepath.Abs("bundle")
	if err != nil {
		return err
	}
	log.Infof("Downloading vulcand into %s", bundleDir)
	if err := runGo(bundleDir, "get", "github.com/mailgun/vulcand"); err != nil {
		return err
	}
	godepsPath := filepath.Join(bundleDir, "src", "github.com", "mailgun", "vulcand", "Godeps", "Godeps.json")
	data, err := ioutil.ReadFile(godepsPath)
	if err != nil {
		return err
	}
	log.Infof("Reading godeps")
	var godeps *Godeps
	if err := json.Unmarshal(data, &godeps); err != nil {
		return err
	}
	log.Infof("Injecting deps")
	// Remove deps taht are
	for _, godep := range godeps.Deps {
		for _, m := range middlewares {
			if m.ImportPath == godep.ImportPath {

			}
		}
	}
	godeps.Deps = append(godeps.Deps, middlewares...)
	encoded, err := json.MarshalIndent(godeps, "", "\t")
	if err != nil {
		return err
	}
	log.Infof("Writing new godeps")
	if err := ioutil.WriteFile(godepsPath, encoded, 0644); err != nil {
		return err
	}
	log.Infof("Compiling dependencies")
	return nil
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

func parseMiddlewares(in []string) ([]Dependency, error) {
	out := make([]Dependency, len(in))
	for i, val := range in {
		middleware, err := parseMiddleware(val)
		if err != nil {
			return nil, err
		}
		out[i] = *middleware
	}
	return out, nil
}

func parseMiddleware(val string) (*Dependency, error) {
	out := strings.SplitN(val, ":", 2)
	if len(out) != 2 {
		return nil, fmt.Errorf("Expected path <repo-path>:<revision>, got: %s", out)
	}
	return &Dependency{ImportPath: out[0], Rev: out[1], Comment: "Injected by vulcanbundle"}, nil
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

func runGodepGo(gopath string, args ...string) error {
	c := exec.Command("godep", append([]string{"go"}, args...)...)
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
