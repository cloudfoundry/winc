package container

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/winc/sandbox"
	"github.com/Microsoft/hcsshim"
)

func Create(rootfsPath, bundlePath, containerId string) error {
	if err := sandbox.Create(rootfsPath, bundlePath, containerId); err != nil {
		return err
	}

	layerChain, err := ioutil.ReadFile(filepath.Join(bundlePath, "layerchain.json"))
	if err != nil {
		return err
	}

	layers := []string{}
	if err := json.Unmarshal(layerChain, &layers); err != nil {
		return err
	}

	layerInfos := []hcsshim.Layer{}
	for _, layerPath := range layers {
		layerId := filepath.Base(layerPath)
		layerGuid, err := hcsshim.NameToGuid(layerId)
		if err != nil {
			return err
		}

		layerInfos = append(layerInfos, hcsshim.Layer{
			ID:   layerGuid.ToString(),
			Path: layerPath,
		})
	}

	driverInfo := hcsshim.DriverInfo{
		HomeDir: filepath.Dir(bundlePath),
		Flavour: 1,
	}
	volumePath, err := hcsshim.GetLayerMountPath(driverInfo, containerId)
	if err != nil {
		return err
	}

	containerConfig := &hcsshim.ContainerConfig{
		SystemType:        "Container",
		Name:              bundlePath,
		VolumePath:        volumePath,
		Owner:             "winc",
		LayerFolderPath:   bundlePath,
		Layers:            layerInfos,
		MappedDirectories: []hcsshim.MappedDir{},
		EndpointList:      []string{},
	}

	_, err = hcsshim.CreateContainer(containerId, containerConfig)
	if err != nil {
		return err
	}

	return nil
}

func Delete(containerId string) error {
	query := hcsshim.ComputeSystemQuery{
		IDs:    []string{containerId},
		Owners: []string{"winc"},
	}
	cps, err := hcsshim.GetContainers(query)
	if err != nil {
		return err
	}

	containerExists := false
	for _, cp := range cps {
		if cp.ID == containerId {
			containerExists = true
		}
	}
	if !containerExists {
		return fmt.Errorf("container %s does not exist", containerId)
	}

	container, err := hcsshim.OpenContainer(containerId)
	if err != nil {
		return err
	}

	err = container.Terminate()
	if hcsshim.IsPending(err) {
		err = container.WaitTimeout(time.Minute * 5)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if err := sandbox.Delete(cps[0].Name, containerId); err != nil {
		return err
	}

	return nil
}
