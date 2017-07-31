package main

import (
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"

	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:      "delete",
	Usage:     "delete a container volume",
	ArgsUsage: `<container-id>`,
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		storePath := context.GlobalString("store")

		sm := sandbox.NewManager(&hcsclient.HCSClient{}, storePath, containerId)
		return sm.Delete()
	},
}
