package main

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/sandbox"
	"code.cloudfoundry.org/winc/volume"
	"github.com/urfave/cli"
)

var statsCommand = cli.Command{
	Name:      "stats",
	Usage:     "show stats for container volume",
	ArgsUsage: `<container-id>`,
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		containerId := context.Args().First()
		storePath := context.GlobalString("store")

		sm := sandbox.NewManager(&hcs.Client{}, &volume.Limiter{}, &volume.Statser{}, storePath, containerId)
		imageStats, err := sm.Stats()
		if err != nil {
			return err
		}

		output, err := json.Marshal(imageStats)
		if err != nil {
			return err
		}

		fmt.Println(string(output))

		return nil
	},
}
