package sandbox

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/hcsclient"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

type ImageSpec struct {
	RootFs       string   `json:"rootfs,omitempty"`
	LayerFolders []string `json:"layerFolders,omitempty"`
}

type SandboxManager interface {
	Create(rootfs string, diskLimit uint64) (*ImageSpec, error)
	Delete() error
}

//go:generate counterfeiter . Limiter
type Limiter interface {
	SetDiskLimit(volumePath string, size uint64) error
}

type sandboxManager struct {
	hcsClient  hcsclient.Client
	limiter    Limiter
	id         string
	driverInfo hcsshim.DriverInfo
}

func NewManager(hcsClient hcsclient.Client, limiter Limiter, storePath, containerId string) SandboxManager {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: storePath,
		Flavour: 1,
	}

	return &sandboxManager{
		hcsClient:  hcsClient,
		limiter:    limiter,
		id:         containerId,
		driverInfo: driverInfo,
	}
}

func (s *sandboxManager) Create(rootfs string, diskLimit uint64) (*ImageSpec, error) {
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
		return nil, &hcsclient.MissingVolumePathError{Id: s.id}
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

func (s *sandboxManager) Delete() error {
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
