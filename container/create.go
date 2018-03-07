package container

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func (m *Manager) Create(bundlePath string) (*specs.Spec, error) {
	_, err := m.hcsClient.GetContainerProperties(m.id)
	if err == nil {
		return nil, &AlreadyExistsError{Id: m.id}
	}
	if _, ok := err.(*hcs.NotFoundError); !ok {
		return nil, err
	}

	spec, err := m.loadBundle(bundlePath)
	if err != nil {
		return nil, err
	}

	volumePath := spec.Root.Path

	layerInfos := []hcsshim.Layer{}
	for _, layerPath := range spec.Windows.LayerFolders {
		layerId := filepath.Base(layerPath)
		layerGuid, err := m.hcsClient.NameToGuid(layerId)
		if err != nil {
			return nil, err
		}

		layerInfos = append(layerInfos, hcsshim.Layer{
			ID:   layerGuid.ToString(),
			Path: layerPath,
		})
	}

	mappedDirs := []hcsshim.MappedDir{}
	for _, d := range spec.Mounts {
		fileInfo, err := os.Stat(d.Source)
		if err != nil {
			return nil, err
		}
		if !fileInfo.IsDir() {
			logrus.WithField("mount", d.Source).Error("mount is not a directory, ignoring")
			continue
		}

		readOnly, err := m.parseMountOptions(d.Options)
		if err != nil {
			return nil, err
		}

		mappedDirs = append(mappedDirs, hcsshim.MappedDir{
			HostPath:      d.Source,
			ContainerPath: destToWindowsPath(d.Destination),
			ReadOnly:      readOnly,
		})
	}

	containerConfig := hcsshim.ContainerConfig{
		SystemType:        "Container",
		HostName:          spec.Hostname,
		VolumePath:        volumePath,
		LayerFolderPath:   "ignored",
		Layers:            layerInfos,
		MappedDirectories: mappedDirs,
	}

	if spec.Windows != nil {
		if spec.Windows.Resources != nil {
			if spec.Windows.Resources.Memory != nil {
				if spec.Windows.Resources.Memory.Limit != nil {
					memBytes := *spec.Windows.Resources.Memory.Limit
					containerConfig.MemoryMaximumInMB = int64(memBytes / 1024 / 1024)
				}
			}
			if spec.Windows.Resources.CPU != nil {
				if spec.Windows.Resources.CPU.Shares != nil {
					containerConfig.ProcessorWeight = uint64(*spec.Windows.Resources.CPU.Shares)
				}
			}
		}

		if spec.Windows.Network != nil {
			if spec.Windows.Network.NetworkSharedContainerName != "" {
				containerConfig.NetworkSharedContainerName = spec.Windows.Network.NetworkSharedContainerName
				containerConfig.Owner = spec.Windows.Network.NetworkSharedContainerName
				endpoint, err := m.hcsClient.GetHNSEndpointByName(spec.Windows.Network.NetworkSharedContainerName)
				if err != nil {
					return nil, err
				}
				containerConfig.EndpointList = []string{endpoint.Id}
			}
		}
	}

	container, err := m.hcsClient.CreateContainer(m.id, &containerConfig)
	if err != nil {
		return nil, err
	}

	cleanupContainer := func() {
		if deleteErr := m.deleteContainer(m.id, container); deleteErr != nil {
			logrus.Error(deleteErr.Error())
		}
	}

	if err := container.Start(); err != nil {
		cleanupContainer()
		return nil, err
	}

	pid, err := m.processManager.ContainerPid(m.id)
	if err != nil {
		cleanupContainer()
		return nil, err
	}

	if err := m.mounter.Mount(pid, volumePath); err != nil {
		cleanupContainer()
		return nil, err
	}

	if err := m.stateManager.Initialize(bundlePath); err != nil {
		cleanupContainer()
		return nil, err
	}

	return spec, nil
}

func (m *Manager) parseMountOptions(options []string) (bool, error) {
	hasReadOnly := false
	hasReadWrite := false
	for _, option := range options {
		if option == "rw" {
			hasReadWrite = true
		} else if option == "ro" {
			hasReadOnly = true
		}
	}

	if hasReadOnly && hasReadWrite {
		return false, &InvalidMountOptionsError{Id: m.id, Options: options}
	}

	readOnly := true
	if hasReadWrite {
		readOnly = false
	}

	return readOnly, nil
}

func destToWindowsPath(input string) string {
	vol := filepath.VolumeName(input)
	if vol == "" {
		input = filepath.Join("C:", input)
	}
	return filepath.Clean(input)
}
