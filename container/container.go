package container

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/winc/container/config"
	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const destroyTimeout = time.Minute

type Manager struct {
	logger         *logrus.Entry
	hcsClient      HCSClient
	mounter        Mounter
	stateManager   StateManager
	id             string
	rootDir        string
	processManager ProcessManager
}

//go:generate counterfeiter -o fakes/mounter.go --fake-name Mounter . Mounter
type Mounter interface {
	Mount(pid int, volumePath string) error
	Unmount(pid int) error
}

//go:generate counterfeiter -o fakes/state_manager.go --fake-name StateManager . StateManager
type StateManager interface {
	Get() (string, string, error)
	Initialize(string) error
	SetRunning(int) error
	SetExecFailed() error
	WriteContainerState(state.ContainerState) error
}

//go:generate counterfeiter -o fakes/process_manager.go --fake-name ProcessManager . ProcessManager
type ProcessManager interface {
	ContainerPid(string) (int, error)
	ProcessStartTime(uint32) (syscall.Filetime, error)
}

//go:generate counterfeiter -o fakes/hcsclient.go --fake-name HCSClient . HCSClient
type HCSClient interface {
	GetContainers(hcsshim.ComputeSystemQuery) ([]hcsshim.ContainerProperties, error)
	GetContainerProperties(string) (hcsshim.ContainerProperties, error)
	NameToGuid(string) (hcsshim.GUID, error)
	CreateContainer(string, *hcsshim.ContainerConfig) (hcs.Container, error)
	OpenContainer(string) (hcs.Container, error)
	IsPending(error) bool
	GetHNSEndpointByName(string) (*hcsshim.HNSEndpoint, error)
}

func NewManager(logger *logrus.Entry, hcsClient HCSClient, mounter Mounter, stateManager StateManager, id, rootDir string, processManager ProcessManager) *Manager {
	return &Manager{
		logger:         logger,
		hcsClient:      hcsClient,
		mounter:        mounter,
		stateManager:   stateManager,
		id:             id,
		rootDir:        rootDir,
		processManager: processManager,
	}
}

func (m *Manager) deleteContainer(containerId string, container hcs.Container) error {
	props, err := m.hcsClient.GetContainerProperties(containerId)
	if err != nil {
		return err
	}

	if props.Stopped {
		if err := container.Close(); err != nil {
			return err
		}
	} else {
		if err := m.shutdownContainer(container); err != nil {
			if err := m.terminateContainer(container); err != nil {
				return err
			}
		}
	}

	return os.RemoveAll(filepath.Join(m.rootDir, m.id))
}

func (m *Manager) shutdownContainer(container hcs.Container) error {
	if err := container.Shutdown(); err != nil {
		if m.hcsClient.IsPending(err) {
			if err := container.WaitTimeout(destroyTimeout); err != nil {
				logrus.Error("hcsContainer.WaitTimeout error after Shutdown", err)
				return err
			}
		} else {
			logrus.Error("hcsContainer.Shutdown error", err)
			return err
		}
	}

	return nil
}

func (m *Manager) terminateContainer(container hcs.Container) error {
	if err := container.Terminate(); err != nil {
		if m.hcsClient.IsPending(err) {
			if err := container.WaitTimeout(destroyTimeout); err != nil {
				logrus.Error("hcsContainer.WaitTimeout error after Terminate", err)
				return err
			}
		} else {
			logrus.Error("hcsContainer.Terminate error", err)
			return err
		}
	}

	return nil
}

func (m *Manager) loadBundle(bundlePath string) (*specs.Spec, error) {
	if bundlePath == "" {
		var err error
		bundlePath, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	bundlePath = filepath.Clean(bundlePath)

	spec, err := config.ValidateBundle(m.logger, bundlePath)
	if err != nil {
		return nil, err
	}

	if _, err := config.ValidateProcess(m.logger, "", spec.Process); err != nil {
		return nil, err
	}

	if filepath.Base(bundlePath) != m.id {
		return nil, &InvalidIdError{Id: m.id}
	}

	return spec, nil
}
