package main

import (
	"fmt"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/hcsprocess"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/container/winsyscall"
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
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		rootDir := context.GlobalString("root")
		pidFile := context.String("pid-file")

		logger := logrus.WithFields(logrus.Fields{
			"containerId": containerId,
			"pidFile":     pidFile,
		})
		logger.Debug("starting process in container")

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, containerId)

		wsc := winsyscall.WinSyscall{}
		sm := state.New(logger, &client, &wsc, containerId, rootDir)
		m := mount.Mounter{}

		ociState, err := sm.State()
		if err != nil {
			return err
		}

		if ociState.Status != "created" {
			return fmt.Errorf("cannot start a container in the %s state", ociState.Status)
		}

		spec, err := cm.Spec(ociState.Bundle)
		if err != nil {
			return err
		}

		process, err := cm.Exec(spec.Process, false)
		if err != nil {
			if cErr, ok := err.(*container.CouldNotCreateProcessError); ok {
				if sErr := sm.Set(nil, true); sErr != nil {
					logger.Error(sErr)
				}
				return cErr
			}
			return err
		}
		defer process.Close()

		if err := sm.Set(process, false); err != nil {
			return err
		}

		if err := m.Mount(process.Pid(), spec.Root.Path); err != nil {
			return err
		}

		wrappedProcess := hcsprocess.New(process)
		return wrappedProcess.WritePIDFile(pidFile)
	},

	SkipArgReorder: true,
}
