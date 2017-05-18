package main

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"
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

		client := hcsclient.HCSClient{}
		cp, err := client.GetContainerProperties(containerId)
		if err != nil {
			return err
		}
		sm := sandbox.NewManager(&client, cp.Name)
		cm := container.NewManager(&client, sm, containerId)
		state, err := cm.State()
		if err != nil {
			return err
		}

		stateJson, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}

		os.Stdout.Write(stateJson)
		return nil
	},
}
