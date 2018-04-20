package main

import (
	"os"
	"strings"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/hcsprocess"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/container/winsyscall"
	"code.cloudfoundry.org/winc/hcs"
	"github.com/pkg/errors"
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

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, containerId)

		wsc := winsyscall.WinSyscall{}
		sm := state.New(logger, &client, &wsc, containerId, rootDir)
		m := mount.Mounter{}

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

		process, err := cm.Exec(spec.Process, !detach)
		if err != nil {
			if cErr, ok := errors.Cause(err).(*container.CouldNotCreateProcessError); ok {
				if sErr := sm.SetFailure(); sErr != nil {
					logger.Error(sErr)
				}
				return cErr
			}
			return err
		}
		defer process.Close()

		if err := sm.SetSuccess(process); err != nil {
			return err
		}

		if err := m.Mount(process.Pid(), spec.Root.Path); err != nil {
			return err
		}

		wrappedProcess := hcsprocess.New(process)
		if err := wrappedProcess.WritePIDFile(pidFile); err != nil {
			return err
		}

		if !detach {
			s := make(chan os.Signal, 1)
			wrappedProcess.SetInterrupt(s)

			exitCode, attachErr := wrappedProcess.AttachIO(os.Stdin, os.Stdout, os.Stderr)

			var errs []string

			ociState, err := sm.State()
			if err != nil {
				logger.Error(err)
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

			if err := cm.Delete(false); err != nil {
				logger.Error(err)
				errs = append(errs, err.Error())
			}

			if attachErr != nil {
				return attachErr
			}

			if len(errs) != 0 {
				return errors.New(strings.Join(errs, "\n"))
			}

			os.Exit(exitCode)
		}

		return nil
	},
	SkipArgReorder: true,
}
