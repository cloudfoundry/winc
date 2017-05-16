package sandbox

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim"
)

func Create(baseImage, sandboxLayer, containerId string) error {
	if _, err := os.Stat(sandboxLayer); err != nil {
		return err
	}

	parentLayerChain, err := ioutil.ReadFile(filepath.Join(baseImage, "layerchain.json"))
	if err != nil {
		return err
	}

	parentLayers := []string{}
	if err := json.Unmarshal(parentLayerChain, &parentLayers); err != nil {
		return err
	}

	driverInfo := hcsshim.DriverInfo{
		HomeDir: filepath.Dir(sandboxLayer),
		Flavour: 1,
	}

	if err := hcsshim.CreateSandboxLayer(driverInfo, containerId, parentLayers[0], parentLayers); err != nil {
		return err
	}

	if err := hcsshim.ActivateLayer(driverInfo, containerId); err != nil {
		return err
	}

	if err := hcsshim.PrepareLayer(driverInfo, containerId, parentLayers); err != nil {
		return err
	}

	sandboxLayers := append([]string{baseImage}, parentLayers...)
	sandboxLayerChain, err := json.Marshal(sandboxLayers)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(sandboxLayer, "layerchain.json"), sandboxLayerChain, 0755); err != nil {
		return err
	}

	return nil
}

func Delete(sandboxLayer, containerId string) error {
	driverInfo := hcsshim.DriverInfo{
		HomeDir: filepath.Dir(sandboxLayer),
		Flavour: 1,
	}

	if err := hcsshim.UnprepareLayer(driverInfo, containerId); err != nil {
		return err
	}

	if err := hcsshim.DeactivateLayer(driverInfo, containerId); err != nil {
		return err
	}

	if err := hcsshim.DestroyLayer(driverInfo, containerId); err != nil {
		return err
	}

	return nil
}
