package container

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const destroyTimeout = time.Minute

type Manager struct {
	hcsClient      HCSClient
	mounter        Mounter
	networkManager NetworkManager
	rootPath       string
	bundlePath     string
	id             string
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
	} `json:"data,omitempty"`
}

//go:generate counterfeiter . Mounter
type Mounter interface {
	Mount(pid int, volumePath string) error
	Unmount(pid int) error
}

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	GetContainerProperties(string) (hcsshim.ContainerProperties, error)
	NameToGuid(string) (hcsshim.GUID, error)
	CreateContainer(string, *hcsshim.ContainerConfig) (hcs.Container, error)
	OpenContainer(string) (hcs.Container, error)
	IsPending(error) bool
}

//go:generate counterfeiter . NetworkManager
type NetworkManager interface {
	AttachEndpointToConfig(hcsshim.ContainerConfig, string) (hcsshim.ContainerConfig, error)
	DeleteContainerEndpoints(hcs.Container, string) error
	DeleteEndpointsById([]string, string) error
}

func NewManager(hcsClient HCSClient, mounter Mounter, networkManager NetworkManager, rootPath, bundlePath string) *Manager {
	return &Manager{
		hcsClient:      hcsClient,
		mounter:        mounter,
		networkManager: networkManager,
		bundlePath:     bundlePath,
		rootPath:       rootPath,
		id:             filepath.Base(bundlePath),
	}
}

func (c *Manager) Create(spec *specs.Spec) error {
	_, err := c.hcsClient.GetContainerProperties(c.id)
	if err == nil {
		return &AlreadyExistsError{Id: c.id}
	}
	if _, ok := err.(*hcs.NotFoundError); !ok {
		return err
	}

	volumePath := spec.Root.Path
	if volumePath == "" {
		return &MissingVolumePathError{Id: c.id}
	}

	layerInfos := []hcsshim.Layer{}
	for _, layerPath := range spec.Windows.LayerFolders {
		layerId := filepath.Base(layerPath)
		layerGuid, err := c.hcsClient.NameToGuid(layerId)
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

		mappedDirs = append(mappedDirs, hcsshim.MappedDir{
			HostPath:      d.Source,
			ContainerPath: destToWindowsPath(d.Destination),
			ReadOnly:      true,
		})
	}

	sandboxDir := filepath.Join(c.rootPath, c.id)

	containerConfig := hcsshim.ContainerConfig{
		SystemType:        "Container",
		Name:              c.bundlePath,
		VolumePath:        volumePath,
		Owner:             "winc",
		LayerFolderPath:   sandboxDir,
		Layers:            layerInfos,
		MappedDirectories: mappedDirs,
	}

	containerConfig, err = c.networkManager.AttachEndpointToConfig(containerConfig, c.id)
	if err != nil {
		return err
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
	}

	container, err := c.hcsClient.CreateContainer(c.id, &containerConfig)
	if err != nil {
		if deleteErr := c.networkManager.DeleteEndpointsById(containerConfig.EndpointList, c.id); deleteErr != nil {
			logrus.Error(deleteErr.Error())
		}

		return err
	}

	if err := container.Start(); err != nil {
		if deleteErr := c.deleteContainer(container); deleteErr != nil {
			logrus.Error(deleteErr.Error())
		}
		return err
	}

	pid, err := c.containerPid(c.id)
	if err != nil {
		if deleteErr := c.deleteContainer(container); deleteErr != nil {
			logrus.Error(deleteErr.Error())
		}
		return err
	}

	if err := c.mounter.Mount(pid, volumePath); err != nil {
		if deleteErr := c.deleteContainer(container); deleteErr != nil {
			logrus.Error(deleteErr.Error())
		}
		return err
	}

	return nil
}

func (c *Manager) Delete() error {
	pid, err := c.containerPid(c.id)
	if err != nil {
		return err
	}

	unmountErr := c.mounter.Unmount(pid)
	if unmountErr != nil {
		logrus.Error(unmountErr.Error())
	}

	container, err := c.hcsClient.OpenContainer(c.id)
	if err != nil {
		return err
	}

	err = c.deleteContainer(container)
	if err != nil {
		return err
	}

	return unmountErr
}

func (c *Manager) State() (*specs.State, error) {
	cp, err := c.hcsClient.GetContainerProperties(c.id)
	if err != nil {
		return nil, err
	}

	var status string
	if cp.Stopped {
		status = "stopped"
	} else {
		status = "created"
	}

	pid, err := c.containerPid(c.id)
	if err != nil {
		return nil, err
	}

	return &specs.State{
		Version: specs.Version,
		ID:      c.id,
		Status:  status,
		Bundle:  c.bundlePath,
		Pid:     pid,
	}, nil
}

func (c *Manager) Exec(processSpec *specs.Process, createIOPipes bool) (hcsshim.Process, error) {
	container, err := c.hcsClient.OpenContainer(c.id)
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
		return nil, &CouldNotCreateProcessError{Id: c.id, Command: command}
	}

	return p, nil
}

func (c *Manager) Stats() (Statistics, error) {
	var stats Statistics

	container, err := c.hcsClient.OpenContainer(c.id)
	if err != nil {
		return stats, err
	}

	containerStats, err := container.Statistics()
	if err != nil {
		return stats, err
	}

	stats.Data.Memory.Raw.TotalRss = containerStats.Memory.UsageCommitBytes
	stats.Data.CPUStats.CPUUsage.Usage = containerStats.Processor.TotalRuntime100ns * 100
	stats.Data.CPUStats.CPUUsage.User = containerStats.Processor.RuntimeUser100ns * 100
	stats.Data.CPUStats.CPUUsage.System = containerStats.Processor.RuntimeKernel100ns * 100

	return stats, nil
}

func (c *Manager) containerPid(id string) (int, error) {
	container, err := c.hcsClient.OpenContainer(id)
	if err != nil {
		return -1, err
	}

	pl, err := container.ProcessList()
	if err != nil {
		return -1, err
	}

	var process hcsshim.ProcessListItem
	oldestTime := time.Now()
	for _, v := range pl {
		if v.ImageName == "wininit.exe" && v.CreateTimestamp.Before(oldestTime) {
			oldestTime = v.CreateTimestamp
			process = v
		}
	}

	return int(process.ProcessId), nil
}

func (c *Manager) deleteContainer(container hcs.Container) error {
	if err := c.networkManager.DeleteContainerEndpoints(container, c.id); err != nil {
		logrus.Error(err.Error())
	}

	props, err := c.hcsClient.GetContainerProperties(c.id)
	if err != nil {
		return err
	}

	if props.Stopped {
		if err := container.Close(); err != nil {
			return err
		}
	} else {
		if err := c.shutdownContainer(container); err != nil {
			if err := c.terminateContainer(container); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Manager) shutdownContainer(container hcs.Container) error {
	if err := container.Shutdown(); err != nil {
		if c.hcsClient.IsPending(err) {
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

func (c *Manager) terminateContainer(container hcs.Container) error {
	if err := container.Terminate(); err != nil {
		if c.hcsClient.IsPending(err) {
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
