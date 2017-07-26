package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/lib/filelock"
	"code.cloudfoundry.org/winc/lib/serial"
	"code.cloudfoundry.org/winc/mounter"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/port_allocator"
	"code.cloudfoundry.org/winc/sandbox"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	specConfig = "config.json"
	usage      = `Open Container Initiative runtime for Windows

winc is a command line client for running applications on Windows packaged
according to the Open Container Initiative (OCI) format and is a compliant
implementation of the Open Container Initiative specification.`
	exactArgs = iota
	minArgs
	maxArgs
)

// version will be populated by the build flags, read from
// VERSION file of the source code.
var version = ""

// gitCommit will be the hash that the binary was built from
// and will be populated by the build flags
var gitCommit = ""

func main() {
	app := cli.NewApp()
	app.Name = "winc.exe"
	app.Usage = usage

	var v []string
	if version != "" {
		v = append(v, version)
	}
	if gitCommit != "" {
		v = append(v, fmt.Sprintf("commit: %s", gitCommit))
	}
	v = append(v, fmt.Sprintf("spec: %s", specs.Version))
	app.Version = strings.Join(v, "\n")

	app.Flags = []cli.Flag{
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
			Value: "text",
			Usage: "set the format used by logs ('text' (default), or 'json')",
		},
		cli.StringFlag{
			Name:  "newuidmap",
			Value: "newuidmap",
			Usage: "ignored",
		},
		cli.StringFlag{
			Name:  "newgidmap",
			Value: "newgidmap",
			Usage: "ignored",
		},
	}

	app.Commands = []cli.Command{
		createCommand,
		deleteCommand,
		stateCommand,
		execCommand,
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
			logWriter = ioutil.Discard
		} else {
			f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0666)
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
			logrus.SetFormatter(new(logrus.JSONFormatter))
		default:
			return &InvalidLogFormatError{Format: logFormat}
		}

		return nil
	}

	cli.ErrWriter = &fatalWriter{cli.ErrWriter}
	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}

type fatalWriter struct {
	cliErrWriter io.Writer
}

func (f *fatalWriter) Write(p []byte) (n int, err error) {
	logrus.Error(string(p))
	return f.cliErrWriter.Write(p)
}

func checkArgs(context *cli.Context, expected, checkType int) error {
	var err error
	cmdName := context.Command.Name
	switch checkType {
	case exactArgs:
		if context.NArg() != expected {
			err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
		}
	case minArgs:
		if context.NArg() < expected {
			err = fmt.Errorf("%s: %q requires a minimum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	case maxArgs:
		if context.NArg() > expected {
			err = fmt.Errorf("%s: %q requires a maximum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	}

	if err != nil {
		fmt.Printf("Incorrect Usage.\n\n")
		cli.ShowCommandHelp(context, cmdName)
		return err
	}
	return nil
}

func fatal(err error) {
	logrus.Error(err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func wireContainerManager(bundlePath, containerId string) (container.ContainerManager, error) {
	client := hcsclient.HCSClient{}

	if bundlePath == "" {
		cp, err := client.GetContainerProperties(containerId)
		if err != nil {
			return nil, err
		}
		bundlePath = cp.Name
	}

	if filepath.Base(bundlePath) != containerId {
		return nil, &hcsclient.InvalidIdError{Id: containerId}
	}

	depotDir := filepath.Dir(bundlePath)
	sm := sandbox.NewManager(&client, &mounter.Mounter{}, depotDir, containerId)

	tracker := &port_allocator.Tracker{
		StartPort: 40000,
		Capacity:  5000,
	}

	locker := filelock.NewLocker("C:\\var\\vcap\\data\\winc\\port-state.json")

	pa := &port_allocator.PortAllocator{
		Tracker:    tracker,
		Serializer: &serial.Serial{},
		Locker:     locker,
	}

	nm := network.NewNetworkManager(&client, pa)

	return container.NewManager(&client, sm, nm, bundlePath), nil
}
