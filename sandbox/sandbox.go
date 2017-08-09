package sandbox

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const CREATE_ATTEMPTS = 5
const DESTROY_ATTEMPTS = 10

type ImageSpec struct {
	RootFs string `json:"rootfs,omitempty"`
	specs.Spec
}

//go:generate counterfeiter . Limiter
type Limiter interface {
	SetDiskLimit(volumePath string, size uint64) error
}

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	CreateLayer(hcsshim.DriverInfo, string, string, []string) (string, error)
	LayerExists(hcsshim.DriverInfo, string) (bool, error)
	DestroyLayer(hcsshim.DriverInfo, string) error
	Retryable(error) bool
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

	exists, err := s.hcsClient.LayerExists(s.driverInfo, s.id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &LayerExistsError{Id: s.id}
	}

	parentLayers := []string{}
	if err := json.Unmarshal(parentLayerChain, &parentLayers); err != nil {
		return nil, &InvalidRootfsLayerChainError{Msg: rootfs}
	}
	sandboxLayers := append([]string{rootfs}, parentLayers...)

	var volumePath string
	var createErr error
	for i := 0; i < CREATE_ATTEMPTS; i++ {
		volumePath, createErr = s.hcsClient.CreateLayer(s.driverInfo, s.id, rootfs, sandboxLayers)
		if createErr == nil || !s.hcsClient.Retryable(createErr) {
			break
		}
	}
	if createErr != nil {
		_ = s.Delete()
		return nil, createErr
	}

	if err := s.limiter.SetDiskLimit(volumePath, diskLimit); err != nil {
		_ = s.Delete()
		return nil, err
	}

	return &ImageSpec{
		RootFs: volumePath,
		Spec: specs.Spec{
			Root: &specs.Root{
				Path: volumePath,
			},
			Windows: &specs.Windows{
				LayerFolders: sandboxLayers,
			},
		},
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

	var destroyErr error
	for i := 0; i < DESTROY_ATTEMPTS; i++ {
		destroyErr = s.hcsClient.DestroyLayer(s.driverInfo, s.id)
		if destroyErr == nil || !s.hcsClient.Retryable(destroyErr) {
			break
		}
	}

	return destroyErr
}
