package main

import (
	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/container/process"
	"code.cloudfoundry.org/winc/hcs"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name:  "run",
	Usage: "create and run a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The run command creates an instance of a container for a bundle. The bundle
is a directory with a specification file named "config.json" and a root
filesystem.

The specification file includes an args parameter. The args parameter is used
to specify command(s) that get run when the container is started. To change the
command(s) that get executed on start, edit the args parameter of the spec.`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: `path to the root of the bundle directory, defaults to the current directory`,
		},
		cli.BoolFlag{
			Name:  "detach, d",
			Usage: "detach from the container's process",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
		cli.BoolFlag{
			Name:  "no-new-keyring",
			Usage: "ignored",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		rootDir := context.GlobalString("root")
		bundlePath := context.String("bundle")
		detach := context.Bool("detach")
		pidFile := context.String("pid-file")

		logger := logrus.WithFields(logrus.Fields{
			"bundle":      bundlePath,
			"containerId": containerId,
			"pidFile":     pidFile,
			"detach":      detach,
		})
		logger.Debug("creating container")

		_, err := createContainer(logger, bundlePath, containerId, pidFile, rootDir)
		if err != nil {
			return err
		}

		cm := container.NewManager(logger, &hcs.Client{}, &mount.Mounter{}, &process.Client{}, containerId, rootDir)
		process, err := cm.Start(detach, "")
		if err != nil {
			return err
		}
		defer process.Close()

		return manageProcess(process, detach, "", cm, true)
	},
	SkipArgReorder: true,
}
