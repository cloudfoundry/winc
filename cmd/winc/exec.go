package main

import (
	"io/ioutil"
	"strconv"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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

       # runc exec <container-id> ps`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
		// 	cli.StringFlag{
		// 		Name:  "cwd",
		// 		Usage: "current working directory in the container",
		// 	},
		// 	cli.StringSliceFlag{
		// 		Name:  "env, e",
		// 		Usage: "set environment variables",
		// 	},
		// 	cli.BoolFlag{
		// 		Name:  "tty, t",
		// 		Usage: "allocate a pseudo-TTY",
		// 	},
		// 	cli.StringFlag{
		// 		Name:  "user, u",
		// 		Usage: "UID (format: <uid>[:<gid>])",
		// 	},
		// 	cli.StringFlag{
		// 		Name:  "process, p",
		// 		Usage: "path to the process.json",
		// 	},
		// 	cli.BoolFlag{
		// 		Name:  "detach,d",
		// 		Usage: "detach from the container's process",
		// 	},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		pidFile := context.String("pid-file")

		processConfig := &specs.Process{
			Args: context.Args()[1:],
		}

		client := hcsclient.HCSClient{}
		cp, err := client.GetContainerProperties(containerId)
		if err != nil {
			return err
		}
		sm := sandbox.NewManager(&client, cp.Name)
		cm := container.NewManager(&client, sm, containerId)

		pid, err := cm.Exec(processConfig)
		if err != nil {
			return err
		}

		if pidFile != "" {
			if err := ioutil.WriteFile(pidFile, []byte(strconv.FormatInt(int64(pid), 10)), 0755); err != nil {
				return err
			}
		}

		return nil
	},
	SkipArgReorder: true,
}
