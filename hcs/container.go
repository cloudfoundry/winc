package hcs

import (
	"time"

	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter -o fakes/container.go --fake-name Container . Container
type Container interface {
	Start() error
	Shutdown() error
	Terminate() error
	Wait() error
	WaitTimeout(time.Duration) error
	Pause() error
	Resume() error
	HasPendingUpdates() (bool, error)
	Statistics() (hcsshim.Statistics, error)
	ProcessList() ([]hcsshim.ProcessListItem, error)
	MappedVirtualDisks() (map[int]hcsshim.MappedVirtualDiskController, error)
	CreateProcess(c *hcsshim.ProcessConfig) (hcsshim.Process, error)
	OpenProcess(pid int) (hcsshim.Process, error)
	Close() error
	Modify(config *hcsshim.ResourceModificationRequestResponse) error
}
