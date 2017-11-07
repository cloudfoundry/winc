package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/winc/endpoint"
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/lib/filelock"
	"code.cloudfoundry.org/winc/lib/serial"
	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/netsh"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/port_allocator"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "winc-network.exe"
	app.Usage = "winc-network is a command line client for managing container networks"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "action",
			Usage: "network action e.g. up,down,create,delete",
			Value: "",
		},
		cli.StringFlag{
			Name:  "configFile",
			Usage: "config file for winc-network",
			Value: "",
		},
		cli.StringFlag{
			Name:  "handle",
			Usage: "container id handle",
			Value: "",
		},
	}
	app.Action = func(context *cli.Context) error {
		config, err := parseConfig(context.String("configFile"))
		if err != nil {
			return fmt.Errorf("configFile: %s", err.Error())
		}
		handle := context.String("handle")
		action := context.String("action")
		if (action == "up" || action == "down") && handle == "" {
			return fmt.Errorf("missing required flag 'handle'")
		}

		networkManager := wireNetworkManager(config, handle)

		switch action {
		case "up":
			var inputs network.UpInputs
			if err := json.NewDecoder(os.Stdin).Decode(&inputs); err != nil {
				return fmt.Errorf("networkUp: %s", err.Error())
			}

			outputs, err := networkManager.Up(inputs)
			if err != nil {
				return fmt.Errorf("networkUp: %s", err.Error())
			}

			if err := json.NewEncoder(os.Stdout).Encode(outputs); err != nil {
				return fmt.Errorf("networkUp: %s", err.Error())
			}

		case "create":
			if err := networkManager.CreateHostNATNetwork(); err != nil {
				return fmt.Errorf("network create: %s", err.Error())
			}

		case "delete":
			if err := networkManager.DeleteHostNATNetwork(); err != nil {
				return fmt.Errorf("network delete: %s", err.Error())
			}

		case "down":
			if err := networkManager.Down(); err != nil {
				return fmt.Errorf("networkDown: %s", err.Error())
			}

		default:
			return fmt.Errorf("invalid action: %s", action)
		}
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}

func parseConfig(configFile string) (network.Config, error) {
	var config network.Config
	if configFile != "" {
		content, err := ioutil.ReadFile(configFile)
		if err != nil {
			return config, err
		}

		if err := json.Unmarshal(content, &config); err != nil {
			return config, err
		}
	}

	return config, nil
}

func wireNetworkManager(config network.Config, handle string) *network.NetworkManager {
	hcsClient := &hcs.Client{}
	runner := netsh.NewRunner(hcsClient, handle)

	tracker := &port_allocator.Tracker{
		StartPort: 40000,
		Capacity:  5000,
	}

	locker := filelock.NewLocker("C:\\var\\vcap\\data\\winc\\port-state.json")

	portAllocator := &port_allocator.PortAllocator{
		Tracker:    tracker,
		Serializer: &serial.Serial{},
		Locker:     locker,
	}

	applier := netrules.NewApplier(runner, handle, config.NetworkName, portAllocator)
	endpointManager := endpoint.NewEndpointManager(hcsClient, handle, config)

	return network.NewNetworkManager(
		hcsClient,
		applier,
		endpointManager,
		handle,
		config,
	)
}

func fatal(err error) {
	logrus.Error(err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
