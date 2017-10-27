package main

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete a container and the resources it holds",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.

EXAMPLE:
For example, if the container id is "windows01" and winc list currently shows the
status of "windows01" as "stopped" the following will delete resources held for
"windows01" removing "windows01" from the winc list of containers:

       # winc delete windows01`,
	// Flags: []cli.Flag{
	// 	cli.BoolFlag{
	// 		Name:  "force, f",
	// 		Usage: "Forcibly deletes the container if it is still running (uses SIGKILL)",
	// 	},
	// },
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		containerId := context.Args().First()

		logrus.WithFields(logrus.Fields{
			"containerId": containerId,
		}).Debug("deleting container")

		cm, err := wireContainerManager("", "", containerId)
		if err != nil {
			return err
		}

		return cm.Delete()
	},
}
