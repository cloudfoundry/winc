package main

import (
	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/hcsprocess"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/hcs"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:  "start",
	Usage: "executes the user defined process in a created container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container`,
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		rootDir := context.GlobalString("root")

		logger := logrus.WithField("containerId", containerId)
		logger.Debug("starting process in container")

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, &mount.Mounter{}, &hcsprocess.Process{}, containerId, rootDir)
		process, err := cm.Start(true)
		if err != nil {
			return err
		}
		defer process.Close()

		return nil
	},

	SkipArgReorder: true,
}
