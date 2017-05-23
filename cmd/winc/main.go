package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	specConfig = "config.json"
	usage      = `Open Container Initiative runtime for Windows

winc is a command line client for running applications on Windows packaged
according to the Open Container Initiative (OCI) format and is a compliant
implementation of the Open Container Initiative specification.`
	exactArgs = iota
	minArgs
	maxArgs
)

func main() {
	app := cli.NewApp()
	app.Name = "winc.exe"
	app.Usage = usage

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "newuidmap",
			Value: "newuidmap",
			Usage: "ignored",
		},
		cli.StringFlag{
			Name:  "newgidmap",
			Value: "newgidmap",
			Usage: "ignored",
		},
	}

	app.Commands = []cli.Command{
		createCommand,
		deleteCommand,
		stateCommand,
		execCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}

func checkArgs(context *cli.Context, expected, checkType int) error {
	var err error
	cmdName := context.Command.Name
	switch checkType {
	case exactArgs:
		if context.NArg() != expected {
			err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
		}
	case minArgs:
		if context.NArg() < expected {
			err = fmt.Errorf("%s: %q requires a minimum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	case maxArgs:
		if context.NArg() > expected {
			err = fmt.Errorf("%s: %q requires a maximum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	}

	if err != nil {
		fmt.Printf("Incorrect Usage.\n\n")
		cli.ShowCommandHelp(context, cmdName)
		return err
	}
	return nil
}

func fatal(err error) {
	logrus.Error(err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
