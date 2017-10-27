package main

import (
	"encoding/json"
	"os"

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

		containerId := context.Args().First()
		showStats := context.Bool("stats")

		logrus.WithFields(logrus.Fields{
			"containerId": containerId,
		}).Debug("retrieving container events and info")

		cm, err := wireContainerManager("", "", containerId)
		if err != nil {
			return err
		}

		if showStats {
			stats, err := cm.Stats()
			if err != nil {
				return err
			}

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
