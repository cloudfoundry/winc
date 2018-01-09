package main

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/image"
	"code.cloudfoundry.org/winc/image/layer"
	"code.cloudfoundry.org/winc/image/volume"
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

		lm := layer.NewManager(hcs.NewClient(), storePath)
		im := image.NewManager(lm, &volume.Limiter{}, &volume.Statser{}, containerId)

		imageStats, err := im.Stats()
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
