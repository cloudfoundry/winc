package container

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/winc/sandbox"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func getContainerProperties(containerId string) (*hcsshim.ContainerProperties, error) {
	query := hcsshim.ComputeSystemQuery{
		IDs:    []string{containerId},
		Owners: []string{"winc"},
	}
	cps, err := hcsshim.GetContainers(query)
	if err != nil {
		return nil, err
	}

	if len(cps) == 0 {
		return nil, &ContainerNotFoundError{Id: containerId}
	}

	if len(cps) > 1 {
		return nil, &ContainerDuplicateError{Id: containerId}
	}

	return &cps[0], nil
}

func Create(rootfsPath, bundlePath, containerId string) error {
	_, err := getContainerProperties(containerId)
	if err == nil {
		return &ContainerExistsError{Id: containerId}
	}
	if _, ok := err.(*ContainerNotFoundError); !ok {
		return err
	}

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
	cp, err := getContainerProperties(containerId)
	if err != nil {
		return err
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

	if err := sandbox.Delete(cp.Name, containerId); err != nil {
		return err
	}

	return nil
}

func State(containerId string) (*specs.State, error) {
	cp, err := getContainerProperties(containerId)
	if err != nil {
		return nil, err
	}

	return &specs.State{
		Version: specs.Version,
		ID:      containerId,
		Status:  "created",
		Bundle:  cp.Name,
	}, nil
}
