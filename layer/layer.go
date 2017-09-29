package layer

import (
	"fmt"
	"os"

	"github.com/Microsoft/hcsshim"
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
	hcsClient  HCSClient
	driverInfo hcsshim.DriverInfo
}

func NewManager(hcsClient HCSClient, storePath string) *Manager {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: storePath,
		Flavour: 1,
	}

	return &Manager{
		hcsClient:  hcsClient,
		driverInfo: driverInfo,
	}
}

func (m *Manager) CreateLayer(id, parentId string, parentLayerPaths []string) (string, error) {
	if err := os.MkdirAll(m.driverInfo.HomeDir, 0755); err != nil {
		return "", err
	}

	var createErr, activateErr, prepareErr error
	for i := 0; i < 3; i++ {
		createErr = m.hcsClient.CreateSandboxLayer(m.driverInfo, id, parentId, parentLayerPaths)
		activateErr = m.hcsClient.ActivateLayer(m.driverInfo, id)
		prepareErr = m.hcsClient.PrepareLayer(m.driverInfo, id, parentLayerPaths)
		if prepareErr == nil {
			break
		}
	}
	if prepareErr != nil {
		return "", fmt.Errorf("failed to create layer (create error: %s, activate error: %s, prepare error: %s)", createErr.Error(), activateErr.Error(), prepareErr.Error())
	}

	volumePath, err := m.hcsClient.GetLayerMountPath(m.driverInfo, id)
	if err != nil {
		return "", err
	} else if volumePath == "" {
		return "", &MissingVolumePathError{Id: id}
	}

	return volumePath, nil
}

func (m *Manager) RemoveLayer(id string) error {
	var unprepareErr, deactivateErr, destroyErr error

	for i := 0; i < 3; i++ {
		unprepareErr = m.hcsClient.UnprepareLayer(m.driverInfo, id)
		deactivateErr = m.hcsClient.DeactivateLayer(m.driverInfo, id)
		destroyErr = m.hcsClient.DestroyLayer(m.driverInfo, id)
		if destroyErr == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to remove layer (unprepare error: %s, deactivate error: %s, destroy error: %s)", unprepareErr.Error(), deactivateErr.Error(), destroyErr.Error())
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
