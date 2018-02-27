package main

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/hcs"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var eventsCommand = cli.Command{
	Name:  "events",
	Usage: "display container events such as OOM notifications, cpu, memory, and IO usage statistics",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container.`,
	Description: `The events command displays information about the container.`,
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "stats", Usage: "display the container's stats then exit"},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		rootDir := context.GlobalString("root")
		containerId := context.Args().First()
		showStats := context.Bool("stats")

		logger := logrus.WithFields(logrus.Fields{
			"containerId": containerId,
		})
		logger.Debug("retrieving container events and info")

		client := hcs.Client{}
		cm := container.NewManager(logger, &client, &mount.Mounter{}, containerId, rootDir)

		stats, err := cm.Stats()
		if err != nil {
			return err
		}

		if showStats {
			statsJson, err := json.MarshalIndent(stats, "", "  ")
			if err != nil {
				return err
			}

			_, err = os.Stdout.Write(statsJson)
			if err != nil {
				return err
			}
		}

		return nil
	},
}
