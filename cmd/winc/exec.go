package main

import (
	"os"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/config"
	"code.cloudfoundry.org/winc/container/hcsprocess"
	"code.cloudfoundry.org/winc/hcs"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "execute new process inside a container",
	ArgsUsage: `<container-id> <command> [command options]  || -p process.json <container-id>

Where "<container-id>" is the name for the instance of the container and
"<command>" is the command to be executed in the container.
"<command>" can't be empty unless a "-p" flag provided.

EXAMPLE:
For example, if the container is configured to run the Windows ps command the
following will output a list of processes running in the container:

       # winc exec <container-id> ps`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
		cli.BoolFlag{
			Name:  "detach,d",
			Usage: "detach from the container's process",
		},
		cli.StringFlag{
			Name:  "process, p",
			Value: "",
			Usage: "path to the process.json",
		},
		cli.StringFlag{
			Name:  "cwd",
			Value: "",
			Usage: "current working directory in the container",
		},
		cli.StringFlag{
			Name:  "user, u",
			Value: "",
			Usage: "Username to execute process as",
		},
		cli.StringSliceFlag{
			Name:  "env, e",
			Usage: "set environment variables",
		},
		// 	cli.BoolFlag{
		// 		Name:  "tty, t",
		// 		Usage: "allocate a pseudo-TTY",
		// 	},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		processConfig := context.String("process")
		args := context.Args()[1:]
		cwd := context.String("cwd")
		user := context.String("user")
		env := context.StringSlice("env")
		pidFile := context.String("pid-file")
		detach := context.Bool("detach")

		logger := logrus.WithField("containerId", containerId)

		processSpec, err := config.ValidateProcess(logger, processConfig, &specs.Process{
			Args: args,
			Cwd:  cwd,
			User: specs.User{
				Username: user,
			},
			Env: env,
		})
		if err != nil {
			return err
		}

		logger = logger.WithFields(logrus.Fields{
			"processConfig": processConfig,
			"pidFile":       pidFile,
			"args":          processSpec.Args,
			"cwd":           processSpec.Cwd,
			"user":          processSpec.User.Username,
			"env":           env,
			"detach":        detach,
		})
		logger.Debug("executing process in container")

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, containerId)

		p, err := cm.Exec(processSpec, !detach)
		if err != nil {
			return err
		}
		defer p.Close()

		wrappedProcess := hcsprocess.New(p)
		if err := wrappedProcess.WritePIDFile(pidFile); err != nil {
			return err
		}

		if !detach {
			s := make(chan os.Signal, 1)
			wrappedProcess.SetInterrupt(s)
			exitCode, err := wrappedProcess.AttachIO(os.Stdin, os.Stdout, os.Stderr)
			if err != nil {
				return err
			}
			os.Exit(exitCode)
		}

		return nil
	},
	SkipArgReorder: true,
}
