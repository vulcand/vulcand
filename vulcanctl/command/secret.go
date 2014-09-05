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
		Name:  "secret",
		Usage: "Operations with vulcan encryption keys",
		Subcommands: []cli.Command{
			{
				Name:   "new_key",
				Usage:  "Generate new seal key",
				Action: cmd.generateKeyAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "file, f", Usage: "File to write to"},
				},
			},
			{
				Name:   "seal_cert",
				Usage:  "Seal certificate",
				Action: cmd.sealCertAction,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "file, f", Usage: "File to write to"},
					cli.StringFlag{Name: "sealKey", Usage: "Seal key - used to encrypt and seal certificate and private key"},
					cli.StringFlag{Name: "privateKey", Usage: "Path to a private key"},
					cli.StringFlag{Name: "cert", Usage: "Path to a certificate"},
				},
			},
		},
	}
}

func (cmd *Command) generateKeyAction(c *cli.Context) {
	key, err := secret.NewKeyString()
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

func (cmd *Command) sealCertAction(c *cli.Context) {
	// Read the key and get a box
	box, err := readBox(c.String("sealKey"))
	if err != nil {
		cmd.printError(err)
		return
	}

	// Read certificate
	stream, closer, err := getStream(c)
	if err != nil {
		cmd.printError(err)
		return
	}
	if closer != nil {
		defer closer.Close()
	}

	cert, err := readCert(c.String("cert"), c.String("privateKey"))
	if err != nil {
		cmd.printError(fmt.Errorf("Failed to read certificate: %s", err))
		return
	}

	bytes, err := sealCert(box, cert)
	if err != nil {
		cmd.printError(fmt.Errorf("Failed to seal certificate: %s", err))
		return
	}

	_, err = stream.Write(bytes)
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
