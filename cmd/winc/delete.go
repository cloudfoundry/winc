package main

import (
	"errors"
	"strings"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/container/winsyscall"
	"code.cloudfoundry.org/winc/hcs"
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
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Do not return an error if <container-id> does not exist",
		},
	},

	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		rootDir := context.GlobalString("root")
		containerId := context.Args().First()
		force := context.Bool("force")

		logger := logrus.WithFields(logrus.Fields{
			"containerId": containerId,
		})
		logger.Debug("deleting container")

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, containerId)

		wsc := winsyscall.WinSyscall{}
		sm := state.New(logger, &client, &wsc, containerId, rootDir)
		m := mount.Mounter{}

		var errs []string

		ociState, err := sm.State()
		if err != nil {
			logger.Error(err)

			if _, ok := err.(*hcs.NotFoundError); ok {
				if force {
					return nil
				}
				return err
			}

			errs = append(errs, err.Error())
		} else if ociState.Pid != 0 {
			if err := m.Unmount(ociState.Pid); err != nil {
				logger.Error(err)
				errs = append(errs, err.Error())
			}
		}

		if err := sm.Delete(); err != nil {
			logger.Error(err)
			errs = append(errs, err.Error())
		}

		if err := cm.Delete(force); err != nil {
			logger.Error(err)
			errs = append(errs, err.Error())
		}

		if len(errs) != 0 {
			return errors.New(strings.Join(errs, "\n"))
		}

		return nil
	},
}
