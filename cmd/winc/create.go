package main

import (
	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/config"
	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/container/winsyscall"
	"code.cloudfoundry.org/winc/hcs"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The create command creates an instance of a container for a bundle. The bundle
	is a directory with a specification file named "` + config.SpecConfig + `" and a root
	filesystem.

	The specification file includes an args parameter. The args parameter is used
	to specify command(s) that get run when the container is started. To change the
	command(s) that get executed on start, edit the args parameter of the spec`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: `path to the root of the bundle directory, defaults to the current directory`,
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

		logger := logrus.WithFields(logrus.Fields{
			"bundle":      bundlePath,
			"containerId": containerId,
		})
		logger.Debug("creating container")

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, containerId)

		wsc := winsyscall.WinSyscall{}
		sm := state.New(logger, &client, &wsc, containerId, rootDir)

		spec, err := cm.Spec(bundlePath)
		if err != nil {
			return err
		}

		if err := cm.Create(spec); err != nil {
			return err
		}

		if err := sm.Initialize(bundlePath); err != nil {
			cm.Delete(true)
			return err
		}

		return nil
	},
}
