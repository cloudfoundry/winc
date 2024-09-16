package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/filelock"
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/endpoint"
	"code.cloudfoundry.org/winc/network/mtu"
	"code.cloudfoundry.org/winc/network/netinterface"
	"code.cloudfoundry.org/winc/network/netrules"
	"code.cloudfoundry.org/winc/network/netsh"
	"code.cloudfoundry.org/winc/network/port_allocator"
	"code.cloudfoundry.org/winc/network/port_allocator/serial"
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
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output for logging",
		},
		cli.StringFlag{
			Name:  "log",
			Value: os.DevNull,
			Usage: "set the log file path where internal debug information is written",
		},
		cli.StringFlag{
			Name:  "log-format",
			Value: "json",
			Usage: "set the format used by logs ('json' (default), or 'text')",
		},
	}
	app.Before = func(context *cli.Context) error {
		debug := context.GlobalBool("debug")
		logFile := context.GlobalString("log")
		logFormat := context.GlobalString("log-format")

		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}

		var logWriter io.Writer
		if logFile == "" || logFile == os.DevNull {
			logWriter = io.Discard
		} else {
			if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
				return err
			}

			f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0644)
			if err != nil {
				return err
			}

			logWriter = f
		}
		logrus.SetOutput(logWriter)

		switch logFormat {
		case "text":
			// retain logrus's default.
		case "json":
			logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: "2006-01-02T15:04:05.000000000Z"})
		default:
			return fmt.Errorf("invalid log format: %s", logFormat)
		}

		return nil
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

		networkManager, err := wireNetworkManager(config, handle)
		if err != nil {
			fatal(err)
		}

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
		content, err := os.ReadFile(configFile)
		if err != nil {
			return config, err
		}

		if err := json.Unmarshal(content, &config); err != nil {
			return config, err
		}
	}

	return config, nil
}

func wireNetworkManager(config network.Config, handle string) (*network.NetworkManager, error) {
	hcsClient := &hcs.Client{}
	runner := netsh.NewRunner(hcsClient, handle, config.WaitTimeoutInSeconds)

	tracker := &port_allocator.Tracker{
		StartPort: 40000,
		Capacity:  5000,
	}

	locker := filelock.NewLocker("C:\\var\\vcap\\data\\winc-network\\port-state.json")

	portAllocator := &port_allocator.PortAllocator{
		Tracker:    tracker,
		Serializer: &serial.Serial{},
		Locker:     locker,
	}

	applier, err := wireApplier(runner, handle, portAllocator)
	if err != nil {
		return nil, err
	}

	endpointManager, err := wireEndpointManager(hcsClient, handle, config)
	if err != nil {
		return nil, err
	}

	m := mtu.New(handle, config.NetworkName, &netinterface.NetInterface{})

	return network.NewNetworkManager(
		hcsClient,
		applier,
		endpointManager,
		handle,
		config,
		m,
	), nil
}

func fatal(err error) {
	logrus.Error(err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func wireApplier(runner *netsh.Runner, handle string, portAlloctor *port_allocator.PortAllocator) (network.NetRuleApplier, error) {
	return netrules.NewApplier(runner, handle, portAlloctor), nil
}

func wireEndpointManager(hcsClient *hcs.Client, handle string, config network.Config) (network.EndpointManager, error) {
	return endpoint.NewEndpointManager(hcsClient, handle, config), nil
}
