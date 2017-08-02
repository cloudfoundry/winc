package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"

	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:      "create",
	Usage:     "create a container volume",
	ArgsUsage: `<rootfs> <container-id>`,
	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "disk-limit-size-bytes",
			Usage: "Disk limit in bytes",
		},
		cli.BoolFlag{
			Name:  "exclude-image-from-quota",
			Usage: "Ignored",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 2, exactArgs); err != nil {
			return err
		}

		rootfsPath := context.Args().First()
		containerId := context.Args().Tail()[0]
		storePath := context.GlobalString("store")

		rootfsPath = filepath.Clean(rootfsPath)
		sm := sandbox.NewManager(&hcsclient.HCSClient{}, storePath, containerId)
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
