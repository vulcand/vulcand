package command

import (
	"fmt"
	"io"
	"os"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/mailgun/vulcand/secret"
)

func NewKeyCommand(cmd *Command) cli.Command {
	return cli.Command{
		Name:  "key",
		Usage: "Operations with vulcan encryption keys",
		Subcommands: []cli.Command{
			{
				Name:   "generate",
				Usage:  "Generate new encryption key",
				Action: cmd.generateKeyAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "file, f", Usage: "File to write to"},
				},
			},
		},
	}
}

func (cmd *Command) generateKeyAction(c *cli.Context) {
	key, err := secret.NewPrintableKey()
	if err != nil {
		cmd.printError(fmt.Errorf("Unable to generate key: %v", err))
		return
	}
	stream, closer, err := getStream(c)
	if err != nil {
		cmd.printError(err)
		return
	}
	if closer != nil {
		defer closer.Close()
	}
	_, err = stream.Write([]byte(key))
	if err != nil {
		cmd.printError(fmt.Errorf("failed writing to output stream, error %s", err))
		return
	}
}

func getStream(c *cli.Context) (io.Writer, io.Closer, error) {
	if c.String("file") != "" {
		file, err := os.OpenFile(c.String("file"), os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open file %s, error: %s", c.String("file"), err)
		}
		return file, file, nil
	}
	return os.Stdout, nil, nil
}
