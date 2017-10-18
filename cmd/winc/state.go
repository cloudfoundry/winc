package main

import (
	"encoding/json"
	"os"

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
		configFile := context.GlobalString("config-file")

		logrus.WithFields(logrus.Fields{
			"containerId": containerId,
		}).Debug("retrieving state of container")

		networkConfig, err := parseConfig(configFile)
		if err != nil {
			return err
		}

		cm, err := wireContainerManager("", "", containerId, networkConfig)
		if err != nil {
			return err
		}

		state, err := cm.State()
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
