package hcs

import (
	"io"
	"time"
)

//go:generate counterfeiter -o fakes/process.go --fake-name Process . Process
type Process interface {
	Pid() int
	Kill() error
	Wait() error
	WaitTimeout(time.Duration) error
	ExitCode() (int, error)
	ResizeConsole(width, height uint16) error
	Stdio() (io.WriteCloser, io.ReadCloser, io.ReadCloser, error)
	CloseStdin() error
	Close() error
}
