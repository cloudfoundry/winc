package sandbox

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/winc/hcsclient"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

type ImageSpec struct {
	RootFs string `json:"rootfs,omitempty"`
	Image  Image  `json:"image,omitempty"`
}

type Image struct {
	Config ImageConfig `json:"config,omitempty"`
}

type ImageConfig struct {
	Layers []string `json:"layers,omitempty"`
}

//go:generate counterfeiter . SandboxManager
type SandboxManager interface {
	Create(rootfs string) (*ImageSpec, error)
	Delete() error
	BundlePath() string
	Mount(pid int, volumePath string) error
	Unmount(pid int) error
}

//go:generate counterfeiter . Mounter
type Mounter interface {
	SetPoint(string, string) error
	DeletePoint(string) error
}

type sandboxManager struct {
	hcsClient  hcsclient.Client
	id         string
	driverInfo hcsshim.DriverInfo
	mounter    Mounter
}

func NewManager(hcsClient hcsclient.Client, mounter Mounter, depotDir, containerId string) SandboxManager {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: depotDir,
		Flavour: 1,
	}

	return &sandboxManager{
		hcsClient:  hcsClient,
		mounter:    mounter,
		id:         containerId,
		driverInfo: driverInfo,
	}
}

func (s *sandboxManager) Create(rootfs string) (*ImageSpec, error) {
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

	return &ImageSpec{
		RootFs: volumePath,
		Image: Image{
			Config: ImageConfig{
				Layers: sandboxLayers,
			},
		},
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

func (s *sandboxManager) BundlePath() string {
	return filepath.Join(s.driverInfo.HomeDir, s.id)
}

func (s *sandboxManager) mountPath(pid int) string {
	return filepath.Join("c:\\", "proc", strconv.Itoa(pid))
}

func (s *sandboxManager) rootPath(pid int) string {
	return filepath.Join(s.mountPath(pid), "root")
}

func (s *sandboxManager) Mount(pid int, volumePath string) error {
	if err := os.MkdirAll(s.rootPath(pid), 0755); err != nil {
		return err
	}

	return s.mounter.SetPoint(s.rootPath(pid), volumePath)
}

func (s *sandboxManager) Unmount(pid int) error {
	if err := s.mounter.DeletePoint(s.rootPath(pid)); err != nil {
		return err
	}

	return os.RemoveAll(s.mountPath(pid))
}
