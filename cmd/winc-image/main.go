package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	exactArgs = iota
	minArgs
	maxArgs
)

func main() {
	app := cli.NewApp()
	app.Name = "winc-image.exe"
	app.Usage = "winc-image is a command line client for managing container volumes"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "store",
			Usage: "Path to the store directory",
			Value: "C:\\var\\lib\\winc-image",
		},
	}

	app.Commands = []cli.Command{
		createCommand,
		deleteCommand,
		statsCommand,
	}

	logrus.SetOutput(os.Stdout)

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
		_ = cli.ShowCommandHelp(context, cmdName)
		return err
	}
	return nil
}
