package main

import (
	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/container/process"
	"code.cloudfoundry.org/winc/hcs"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kills a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.

EXAMPLE:
For example, if the container id is "windows01" and winc list currently shows the
status of "windows01" as "stopped" the following will kill container held for
"windows01" removing "windows01" from the winc list of containers:

       # winc kill windows01`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Do not return an error if <container-id> does not exist",
		},
	},

	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 2, exactArgs); err != nil {
			return err
		}

		rootDir := context.GlobalString("root")
		containerId := context.Args().First()
		signal := context.Args().Get(1)

		logger := logrus.WithFields(logrus.Fields{
			"containerId": containerId,
		})
		logger.Debug("deleting container")

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, &mount.Mounter{}, &process.Client{}, containerId, rootDir)
		return cm.Kill(signal)
	},
}
