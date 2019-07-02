package main

import (
	"os"
	
	"github.com/urfave/cli"
)

var listCommand = cli.Command{
	Name:  "list",
	Usage: "output the list of containers",
	Description: `The list command outputs state information for the list of running containers.`,
	Action: func(context *cli.Context) error {
		err := checkArgs(context, 0, exactArgs);

		if err != nil {
			return err
		}

		_, err = os.Stdout.Write(fmt.Println([]byte("No containers have been created.")))
		return err
	},
}