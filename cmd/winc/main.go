package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime"
	"code.cloudfoundry.org/winc/runtime/container"
	"code.cloudfoundry.org/winc/runtime/hcsprocess"
	"code.cloudfoundry.org/winc/runtime/mount"
	"code.cloudfoundry.org/winc/runtime/state"
	"code.cloudfoundry.org/winc/runtime/winsyscall"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sys/windows"
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

var run *runtime.Runtime

var (
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	getHandleInformation = kernel32.NewProc("GetHandleInformation")
)

type stateFactory struct{}

func (f *stateFactory) NewManager(logger *logrus.Entry, hcsClient *hcs.Client, winSyscall *winsyscall.WinSyscall, id, rootDir string) runtime.StateManager {
	return state.New(logger, hcsClient, winSyscall, id, rootDir)
}

type containerFactory struct{}

func (f *containerFactory) NewManager(logger *logrus.Entry, hcsClient *hcs.Client, id string) runtime.ContainerManager {
	return container.New(logger, hcsClient, id)
}

type processWrapper struct{}

func (w *processWrapper) Wrap(p hcs.Process) runtime.WrappedProcess {
	return hcsprocess.New(p)
}

func main() {
	var logFile *os.File
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

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
		cli.Uint64Flag{
			Name:  "log-handle",
			Usage: "write the logs to this handle that winc has inherited",
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
		logHandle := context.GlobalUint64("log-handle")
		log := context.GlobalString("log")
		logFormat := context.GlobalString("log-format")
		rootDir := context.GlobalString("root")

		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}

		var logWriter io.Writer
		logWriter = ioutil.Discard

		if !emptyLog(log) && logHandle != 0 {
			return errors.New("only one of --log and --log-handle can be passed")
		}

		if logHandle != 0 {
			if err := validHandle(syscall.Handle(logHandle)); err != nil {
				return fmt.Errorf("log handle %d invalid: %s", logHandle, err.Error())
			}

			logFile = os.NewFile(uintptr(logHandle), fmt.Sprintf("%d.winc.log", os.Getpid()))
			logWriter = logFile
		}

		if !emptyLog(log) {
			if err := os.MkdirAll(filepath.Dir(log), 0666); err != nil {
				return err
			}

			var err error
			logFile, err = os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0666)
			if err != nil {
				return err
			}

			logWriter = logFile
		}

		logrus.SetOutput(logWriter)

		switch logFormat {
		case "text":
			// retain logrus's default.
		case "json":
			logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})
		default:
			return &InvalidLogFormatError{Format: logFormat}
		}

		containerFactory := &containerFactory{}
		stateFactory := &stateFactory{}
		mounter := &mount.Mounter{}
		hcsClient := &hcs.Client{}
		processWrapper := &processWrapper{}

		run = runtime.New(stateFactory, containerFactory, mounter, hcsClient, processWrapper, rootDir)
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

func validHandle(handle syscall.Handle) error {
	var flags uint32
	r0, _, err := getHandleInformation.Call(uintptr(handle), uintptr(unsafe.Pointer(&flags)))
	if r0 == 0 {
		return err
	}

	return nil
}

func emptyLog(log string) bool {
	return (log == "" || log == os.DevNull)
}
