package sandbox

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

type ImageSpec struct {
	RootFs       string   `json:"rootfs,omitempty"`
	LayerFolders []string `json:"layerFolders,omitempty"`
}

//go:generate counterfeiter . Limiter
type Limiter interface {
	SetDiskLimit(volumePath string, size uint64) error
}

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	CreateSandboxLayer(hcsshim.DriverInfo, string, string, []string) error
	ActivateLayer(hcsshim.DriverInfo, string) error
	PrepareLayer(hcsshim.DriverInfo, string, []string) error
	GetLayerMountPath(hcsshim.DriverInfo, string) (string, error)
	LayerExists(hcsshim.DriverInfo, string) (bool, error)
	UnprepareLayer(hcsshim.DriverInfo, string) error
	DeactivateLayer(hcsshim.DriverInfo, string) error
	DestroyLayer(hcsshim.DriverInfo, string) error
}

type Manager struct {
	hcsClient  HCSClient
	limiter    Limiter
	id         string
	driverInfo hcsshim.DriverInfo
}

func NewManager(hcsClient HCSClient, limiter Limiter, storePath, containerId string) *Manager {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: storePath,
		Flavour: 1,
	}

	return &Manager{
		hcsClient:  hcsClient,
		limiter:    limiter,
		id:         containerId,
		driverInfo: driverInfo,
	}
}

func (s *Manager) Create(rootfs string, diskLimit uint64) (*ImageSpec, error) {
	err := os.MkdirAll(s.driverInfo.HomeDir, 0755)
	if err != nil {
		return nil, err
	}

	_, err = os.Stat(rootfs)
	if os.IsNotExist(err) {
		return nil, &MissingRootfsError{Msg: rootfs}
	} else if err != nil {
		return nil, err
	}

	parentLayerChain, err := ioutil.ReadFile(filepath.Join(rootfs, "layerchain.json"))
	if err != nil {
		return nil, &MissingRootfsLayerChainError{Msg: rootfs}
	}

	parentLayers := []string{}
	if err := json.Unmarshal(parentLayerChain, &parentLayers); err != nil {
		return nil, &InvalidRootfsLayerChainError{Msg: rootfs}
	}
	sandboxLayers := append([]string{rootfs}, parentLayers...)

	if err := s.hcsClient.CreateSandboxLayer(s.driverInfo, s.id, rootfs, sandboxLayers); err != nil {
		return nil, err
	}

	if err := s.hcsClient.ActivateLayer(s.driverInfo, s.id); err != nil {
		return nil, err
	}

	if err := s.hcsClient.PrepareLayer(s.driverInfo, s.id, sandboxLayers); err != nil {
		return nil, err
	}

	volumePath, err := s.hcsClient.GetLayerMountPath(s.driverInfo, s.id)
	if err != nil {
		return nil, err
	} else if volumePath == "" {
		return nil, &MissingVolumePathError{Id: s.id}
	}

	if err := s.limiter.SetDiskLimit(volumePath, diskLimit); err != nil {
		_ = s.Delete()
		return nil, err
	}

	return &ImageSpec{
		RootFs:       volumePath,
		LayerFolders: sandboxLayers,
	}, nil
}

func (s *Manager) Delete() error {
	exists, err := s.hcsClient.LayerExists(s.driverInfo, s.id)
	if err != nil {
		return err
	}
	if !exists {
		logrus.Warningf("Layer `%s` not found. Skipping delete.", s.id)
		return nil
	}

	if err := s.hcsClient.UnprepareLayer(s.driverInfo, s.id); err != nil {
		return err
	}

	if err := s.hcsClient.DeactivateLayer(s.driverInfo, s.id); err != nil {
		return err
	}

	if err := s.hcsClient.DestroyLayer(s.driverInfo, s.id); err != nil {
		return err
	}

	return nil
}
