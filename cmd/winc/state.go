package main

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/container/winsyscall"
	"code.cloudfoundry.org/winc/hcs"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var stateCommand = cli.Command{
	Name:  "state",
	Usage: "output the state of a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container.`,
	Description: `The state command outputs current state information for the
instance of a container.`,
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		rootDir := context.GlobalString("root")

		logger := logrus.WithFields(logrus.Fields{
			"containerId": containerId,
		})
		logger.Debug("retrieving state of container")

		client := hcs.Client{}
		wsc := winsyscall.WinSyscall{}
		sm := state.New(logger, &client, &wsc, containerId, rootDir)

		state, err := sm.State()
		if err != nil {
			return err
		}

		stateJson, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}

		_, err = os.Stdout.Write(stateJson)
		return err
	},
}
