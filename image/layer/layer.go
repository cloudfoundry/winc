package layer

import (
	"os"

	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	CreateLayer(hcsshim.DriverInfo, string, string, []string) error
	RemoveLayer(hcsshim.DriverInfo, string) error
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

	if err := m.hcsClient.CreateLayer(m.driverInfo, id, parentId, parentLayerPaths); err != nil {
		return "", err
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
	return m.hcsClient.RemoveLayer(m.driverInfo, id)
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
