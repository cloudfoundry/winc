package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var listCommand = cli.Command{
	Name:  "list",
	Usage: "output the list of containers",
	Description: `The list command outputs state information for the list of running containers.`,
	Action: func(context *cli.Context) error {
		if err != nil {
			return err
		}

		_, err = os.Stdout.Write("No containers have been created.")
		return err
	},
}