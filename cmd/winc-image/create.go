package main

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/mounter"
	"code.cloudfoundry.org/winc/sandbox"

	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:      "create",
	Usage:     "create a container volume",
	ArgsUsage: `<rootfs> <container-id>`,
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 2, exactArgs); err != nil {
			return err
		}

		rootfsPath := context.Args().First()
		containerId := context.Args().Tail()[0]

		sm := sandbox.NewManager(&hcsclient.HCSClient{}, &mounter.Mounter{}, depotDir, containerId)
		imageSpec, err := sm.Create(rootfsPath)
		if err != nil {
			return err
		}

		output, err := json.Marshal(&imageSpec)
		if err != nil {
			return err
		}

		fmt.Println(string(output))

		return nil
	},
}
