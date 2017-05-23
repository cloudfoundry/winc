package sandbox

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/hcsclient"

	"github.com/Microsoft/hcsshim"
)

var sandboxFiles = []string{"Hives", "initialized", "sandbox.vhdx", "layerchain.json"}

//go:generate counterfeiter . SandboxManager
type SandboxManager interface {
	Create(rootfs string) error
	Delete() error
	BundlePath() string
}

type sandboxManager struct {
	bundlePath string
	hcsClient  hcsclient.Client
	id         string
	driverInfo hcsshim.DriverInfo
}

func NewManager(hcsClient hcsclient.Client, bundlePath string) SandboxManager {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: filepath.Dir(bundlePath),
		Flavour: 1,
	}

	return &sandboxManager{
		hcsClient:  hcsClient,
		bundlePath: bundlePath,
		id:         filepath.Base(bundlePath),
		driverInfo: driverInfo,
	}
}

func (s *sandboxManager) Create(rootfs string) error {
	_, err := os.Stat(s.bundlePath)
	if os.IsNotExist(err) {
		return &MissingBundlePathError{Msg: s.bundlePath}
	} else if err != nil {
		return err
	}

	_, err = os.Stat(rootfs)
	if os.IsNotExist(err) {
		return &MissingRootfsError{Msg: rootfs}
	} else if err != nil {
		return err
	}

	parentLayerChain, err := ioutil.ReadFile(filepath.Join(rootfs, "layerchain.json"))
	if err != nil {
		return &MissingRootfsLayerChainError{Msg: rootfs}
	}

	parentLayers := []string{}
	if err := json.Unmarshal(parentLayerChain, &parentLayers); err != nil {
		return &InvalidRootfsLayerChainError{Msg: rootfs}
	}

	if err := s.hcsClient.CreateSandboxLayer(s.driverInfo, s.id, parentLayers[0], parentLayers); err != nil {
		return err
	}

	if err := s.hcsClient.ActivateLayer(s.driverInfo, s.id); err != nil {
		return err
	}

	if err := s.hcsClient.PrepareLayer(s.driverInfo, s.id, parentLayers); err != nil {
		return err
	}

	sandboxLayers := append([]string{rootfs}, parentLayers...)
	sandboxLayerChain, err := json.Marshal(sandboxLayers)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(s.bundlePath, "layerchain.json"), sandboxLayerChain, 0755); err != nil {
		return err
	}

	return nil
}

func (s *sandboxManager) Delete() error {
	if err := s.hcsClient.UnprepareLayer(s.driverInfo, s.id); err != nil {
		return err
	}

	if err := s.hcsClient.DeactivateLayer(s.driverInfo, s.id); err != nil {
		return err
	}

	for _, f := range sandboxFiles {
		layerFile := filepath.Join(s.bundlePath, f)
		if err := os.RemoveAll(layerFile); err != nil {
			return &UnableToDestroyLayerError{Msg: layerFile}
		}
	}

	return nil
}

func (s *sandboxManager) BundlePath() string {
	return s.bundlePath
}
