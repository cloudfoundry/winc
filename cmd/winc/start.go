package main

import (
	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/container/process"
	"code.cloudfoundry.org/winc/hcs"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:  "start",
	Usage: "executes the user defined process in a created container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "duplicate-handle-file",
			Usage: "duplicate a handle to the init process into the parent process and persist a handle reference in the file",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		rootDir := context.GlobalString("root")

		logger := logrus.WithField("containerId", containerId)
		logger.Debug("starting process in container")

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, &mount.Mounter{}, &process.Client{}, containerId, rootDir)
		duplicateHandle := context.String("duplicate-handle-file")
		process, err := cm.Start(true, duplicateHandle)
		if err != nil {
			return err
		}
		defer process.Close()

		return nil
	},

	SkipArgReorder: true,
}
