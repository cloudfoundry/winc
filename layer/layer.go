package layer

import (
	"fmt"
	"os"
	"strings"

	"github.com/Microsoft/hcsshim"
)

type createStep int

const (
	noCreateFailure = iota
	createSandboxLayerFailed
	activateLayerFailed
	prepareLayerFailed
	getLayerMountPathFailed
)

type deleteStep int

const (
	noDeleteFailure deleteStep = iota
	unprepareLayerFailed
	deactivateLayerFailed
	destroyLayerFailed
)

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	CreateSandboxLayer(hcsshim.DriverInfo, string, string, []string) error
	ActivateLayer(hcsshim.DriverInfo, string) error
	PrepareLayer(hcsshim.DriverInfo, string, []string) error
	UnprepareLayer(hcsshim.DriverInfo, string) error
	DeactivateLayer(hcsshim.DriverInfo, string) error
	DestroyLayer(hcsshim.DriverInfo, string) error
	LayerExists(hcsshim.DriverInfo, string) (bool, error)
	GetLayerMountPath(hcsshim.DriverInfo, string) (string, error)
}

type Manager struct {
	hcsClient        HCSClient
	driverInfo       hcsshim.DriverInfo
	createFailedStep createStep
	deleteFailedStep deleteStep
}

func NewManager(hcsClient HCSClient, storePath string) *Manager {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: storePath,
		Flavour: 1,
	}

	return &Manager{
		hcsClient:        hcsClient,
		driverInfo:       driverInfo,
		createFailedStep: noCreateFailure,
		deleteFailedStep: noDeleteFailure,
	}
}

func (m *Manager) CreateLayer(id, parentId string, parentLayerPaths []string) (string, error) {
	if err := os.MkdirAll(m.driverInfo.HomeDir, 0755); err != nil {
		return "", err
	}

	var err error
	var volumePath string

	switch m.createFailedStep {
	case noCreateFailure:
		fallthrough
	case createSandboxLayerFailed:
		if err := m.hcsClient.CreateSandboxLayer(m.driverInfo, id, parentId, parentLayerPaths); err != nil {
			m.createFailedStep = createSandboxLayerFailed
			return "", err
		}
		fallthrough

	case activateLayerFailed:
		if err := m.hcsClient.ActivateLayer(m.driverInfo, id); err != nil {
			m.createFailedStep = activateLayerFailed
			return "", err
		}
		fallthrough

	case prepareLayerFailed:
		if err := m.hcsClient.PrepareLayer(m.driverInfo, id, parentLayerPaths); err != nil {
			m.createFailedStep = prepareLayerFailed
			return "", err
		}
		fallthrough

	case getLayerMountPathFailed:
		volumePath, err = m.hcsClient.GetLayerMountPath(m.driverInfo, id)
		if err != nil {
			m.createFailedStep = getLayerMountPathFailed
			return "", err
		} else if volumePath == "" {
			return "", &MissingVolumePathError{Id: id}
		}
	default:
		panic(fmt.Sprintf("invalid create failed step %d", m.createFailedStep))
	}

	return volumePath, nil
}

func (m *Manager) RemoveLayer(id string) error {
	switch m.deleteFailedStep {
	case noDeleteFailure:
		fallthrough
	case unprepareLayerFailed:
		if err := m.hcsClient.UnprepareLayer(m.driverInfo, id); err != nil {
			m.deleteFailedStep = unprepareLayerFailed
			return err
		}
		fallthrough
	case deactivateLayerFailed:
		if err := m.hcsClient.DeactivateLayer(m.driverInfo, id); err != nil {
			m.deleteFailedStep = deactivateLayerFailed
			return err
		}
		fallthrough
	case destroyLayerFailed:
		if err := m.hcsClient.DestroyLayer(m.driverInfo, id); err != nil {
			m.deleteFailedStep = destroyLayerFailed
			return err
		}
	default:
		panic(fmt.Sprintf("invalid delete failed step %d", m.deleteFailedStep))
	}

	return nil
}

func (m *Manager) LayerExists(id string) (bool, error) {
	return m.hcsClient.LayerExists(m.driverInfo, id)
}

func (m *Manager) GetLayerMountPath(id string) (string, error) {
	return m.hcsClient.GetLayerMountPath(m.driverInfo, id)
}

func (m *Manager) HomeDir() string {
	return m.driverInfo.HomeDir
}

func (m *Manager) Retryable(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "This operation returned because the timeout period expired"))
}
