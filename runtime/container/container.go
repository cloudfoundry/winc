package container

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/runtime/config"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const destroyTimeout = time.Minute

type Manager struct {
	logger    *logrus.Entry
	hcsClient HCSClient
	id        string
}

type Statistics struct {
	Data struct {
		CPUStats struct {
			CPUUsage struct {
				Usage  uint64 `json:"total"`
				System uint64 `json:"kernel"`
				User   uint64 `json:"user"`
			} `json:"usage"`
		} `json:"cpu"`
		Memory struct {
			Raw struct {
				TotalRss uint64 `json:"total_rss,omitempty"`
			} `json:"raw,omitempty"`
		} `json:"memory,omitempty"`
		Pids struct {
			Current uint64 `json:"current,omitempty"`
			Limit uint64 `json:"limit,omitempty"`
		} `json:"pids"`
	} `json:"data,omitempty"`
}

//go:generate counterfeiter -o fakes/hcsclient.go --fake-name HCSClient . HCSClient
type HCSClient interface {
	GetContainers(hcsshim.ComputeSystemQuery) ([]hcsshim.ContainerProperties, error)
	GetContainerProperties(string) (hcsshim.ContainerProperties, error)
	NameToGuid(string) (hcsshim.GUID, error)
	CreateContainer(string, *hcsshim.ContainerConfig) (hcs.Container, error)
	OpenContainer(string) (hcs.Container, error)
	IsPending(error) bool
	GetHNSEndpointByName(string) (*hcsshim.HNSEndpoint, error)
}

func New(logger *logrus.Entry, hcsClient HCSClient, id string) *Manager {
	return &Manager{
		logger:    logger,
		hcsClient: hcsClient,
		id:        id,
	}
}

func (m *Manager) Spec(bundlePath string) (*specs.Spec, error) {
	if bundlePath == "" {
		var err error
		bundlePath, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	bundlePath = filepath.Clean(bundlePath)

	spec, err := config.ValidateBundle(m.logger, bundlePath)
	if err != nil {
		return nil, err
	}

	if _, err := config.ValidateProcess(m.logger, "", spec.Process); err != nil {
		return nil, err
	}

	if filepath.Base(bundlePath) != m.id {
		return nil, &InvalidIdError{Id: m.id}
	}

	return spec, nil
}

func (m *Manager) Create(spec *specs.Spec) error {
	_, err := m.hcsClient.GetContainerProperties(m.id)
	if err == nil {
		return &AlreadyExistsError{Id: m.id}
	}
	if _, ok := err.(*hcs.NotFoundError); !ok {
		return err
	}

	layerInfos := []hcsshim.Layer{}
	for _, layerPath := range spec.Windows.LayerFolders {
		layerId := filepath.Base(layerPath)
		layerGuid, err := m.hcsClient.NameToGuid(layerId)
		if err != nil {
			return err
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
			return err
		}
		if !fileInfo.IsDir() {
			logrus.WithField("mount", d.Source).Error("mount is not a directory, ignoring")
			continue
		}
		/*
			readOnly, err := m.parseMountOptions(d.Options)
			if err != nil {
				return err
			}
		*/
		mappedDirs = append(mappedDirs, hcsshim.MappedDir{
			HostPath:      d.Source,
			ContainerPath: destToWindowsPath(d.Destination),
			ReadOnly:      false,
		})
	}

	containerConfig := hcsshim.ContainerConfig{
		SystemType:        "Container",
		HostName:          spec.Hostname,
		VolumePath:        spec.Root.Path,
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
					return err
				}
				containerConfig.EndpointList = []string{endpoint.Id}
			}
		}
	}

	container, err := m.hcsClient.CreateContainer(m.id, &containerConfig)
	if err != nil {
		return err
	}

	if err := container.Start(); err != nil {
		if deleteErr := m.deleteContainer(container); deleteErr != nil {
			logrus.Error(deleteErr.Error())
		}
		return err
	}

	return nil
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

func (m *Manager) Exec(processSpec *specs.Process, createIOPipes bool) (hcs.Process, error) {
	container, err := m.hcsClient.OpenContainer(m.id)
	if err != nil {
		return nil, err
	}

	env := map[string]string{}
	for _, e := range processSpec.Env {
		v := strings.Split(e, "=")
		env[v[0]] = strings.Join(v[1:], "=")
	}

	pc := &hcsshim.ProcessConfig{
		CommandLine:      makeCmdLine(processSpec.Args),
		CreateStdInPipe:  createIOPipes,
		CreateStdOutPipe: createIOPipes,
		CreateStdErrPipe: createIOPipes,
		WorkingDirectory: processSpec.Cwd,
		User:             processSpec.User.Username,
		Environment:      env,
	}
	p, err := container.CreateProcess(pc)
	if err != nil {
		command := ""
		if len(processSpec.Args) != 0 {
			command = processSpec.Args[0]
		}
		finalErr := &CouldNotCreateProcessError{Id: m.id, Command: command}

		cleanedError := hcs.CleanError(err)
		return nil, errors.Wrap(finalErr, cleanedError.Error())
	}

	return p, nil
}

func (m *Manager) Stats() (Statistics, error) {
	var stats Statistics

	container, err := m.hcsClient.OpenContainer(m.id)
	if err != nil {
		return stats, err
	}

	containerStats, err := container.Statistics()
	if err != nil {
		return stats, err
	}

	processListItems, err := container.ProcessList()
	if err != nil {
		return stats, err
	}

	stats.Data.Memory.Raw.TotalRss = containerStats.Memory.UsageCommitBytes
	stats.Data.CPUStats.CPUUsage.Usage = containerStats.Processor.TotalRuntime100ns * 100
	stats.Data.CPUStats.CPUUsage.User = containerStats.Processor.RuntimeUser100ns * 100
	stats.Data.CPUStats.CPUUsage.System = containerStats.Processor.RuntimeKernel100ns * 100
	stats.Data.Pids.Current = uint64(len(processListItems))

	return stats, nil
}

func (m *Manager) Delete(force bool) error {
	container, err := m.hcsClient.OpenContainer(m.id)
	if err != nil {
		if force {
			_, ok := err.(*hcs.NotFoundError)
			if ok {
				return nil
			}
		}

		return err
	}

	return m.deleteContainer(container)
}

func (m *Manager) deleteContainer(container hcs.Container) error {
	props, err := m.hcsClient.GetContainerProperties(m.id)
	if err != nil {
		return err
	}

	if props.Stopped {
		if err := container.Close(); err != nil {
			return err
		}
	} else {
		if err := m.shutdownContainer(container); err != nil {
			if err := m.terminateContainer(container); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) shutdownContainer(container hcs.Container) error {
	if err := container.Shutdown(); err != nil {
		if m.hcsClient.IsPending(err) {
			if err := container.WaitTimeout(destroyTimeout); err != nil {
				logrus.Error("hcsContainer.WaitTimeout error after Shutdown", err)
				return err
			}
		} else {
			logrus.Error("hcsContainer.Shutdown error", err)
			return err
		}
	}

	return nil
}

func (m *Manager) terminateContainer(container hcs.Container) error {
	if err := container.Terminate(); err != nil {
		if m.hcsClient.IsPending(err) {
			if err := container.WaitTimeout(destroyTimeout); err != nil {
				logrus.Error("hcsContainer.WaitTimeout error after Terminate", err)
				return err
			}
		} else {
			logrus.Error("hcsContainer.Terminate error", err)
			return err
		}
	}

	return nil
}

func destToWindowsPath(input string) string {
	vol := filepath.VolumeName(input)
	if vol == "" {
		input = filepath.Join("C:", input)
	}
	return filepath.Clean(input)
}

func makeCmdLine(args []string) string {
	if len(args) > 0 {
		args[0] = filepath.Clean(args[0])
		base := filepath.Base(args[0])
		match, _ := regexp.MatchString(`\.[a-zA-Z]{3}$`, base)
		if !match {
			args[0] += ".exe"
		}
	}
	var s string
	for _, v := range args {
		if s != "" {
			s += " "
		}
		s += syscall.EscapeArg(v)
	}

	return s
}
