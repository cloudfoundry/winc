package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/winc/container"
	"code.cloudfoundry.org/winc/container/hcsprocess"
	"code.cloudfoundry.org/winc/container/mount"
	"code.cloudfoundry.org/winc/hcs"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	usage = `Open Container Initiative runtime for Windows

winc is a command line client for running applications on Windows packaged
according to the Open Container Initiative (OCI) format and is a compliant
implementation of the Open Container Initiative specification.`
	exactArgs = iota
	minArgs
	maxArgs
)

func main() {
	app := cli.NewApp()
	app.Name = "winc.exe"
	app.Usage = usage

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
			Value: "json",
			Usage: "set the format used by logs ('json' (default), or 'text')",
		},
		cli.StringFlag{
			Name:  "image-store",
			Value: "",
			Usage: "ignored",
		},
		cli.StringFlag{
			Name:  "root",
			Value: "C:\\ProgramData\\winc",
			Usage: "directory for storage of container state",
		},
	}

	app.Commands = []cli.Command{
		createCommand,
		deleteCommand,
		runCommand,
		stateCommand,
		startCommand,
		execCommand,
		eventsCommand,
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
			if err := os.MkdirAll(filepath.Dir(logFile), 0666); err != nil {
				return err
			}

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
		_ = cli.ShowCommandHelp(context, cmdName)
		return err
	}
	return nil
}

func fatal(err error) {
	logrus.Error(err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func createContainer(logger *logrus.Entry, bundlePath, containerId, pidFile, rootDir string) (*specs.Spec, error) {
	client := hcs.Client{}
	cm := container.NewManager(logger, &client, &mount.Mounter{}, &hcsprocess.Process{}, containerId, rootDir)

	spec, err := cm.Create(bundlePath)
	if err != nil {
		return nil, err
	}

	if pidFile != "" {
		state, err := cm.State()
		if err != nil {
			return nil, err
		}

		if err := ioutil.WriteFile(pidFile, []byte(strconv.FormatInt(int64(state.Pid), 10)), 0666); err != nil {
			return nil, err
		}
	}

	return spec, nil
}
