package container

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/winc/container/config"
	"code.cloudfoundry.org/winc/container/state"
	"code.cloudfoundry.org/winc/hcs"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const destroyTimeout = time.Minute

type Manager struct {
	logger         *logrus.Entry
	hcsClient      HCSClient
	mounter        Mounter
	state          StateManager
	id             string
	rootDir        string
	processManager ProcessManager
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

//go:generate counterfeiter -o fakes/mounter.go --fake-name Mounter . Mounter
type Mounter interface {
	Mount(pid int, volumePath string) error
	Unmount(pid int) error
}

//go:generate counterfeiter -o fakes/state_manager.go --fake-name StateManager . StateManager
type StateManager interface {
	Get() (*specs.State, error)
	Initialize(string) error
	SetRunning(int) error
	SetExecFailed() error
	WriteContainerState(state.ContainerState) error
}

//go:generate counterfeiter -o fakes/process_manager.go --fake-name ProcessManager . ProcessManager
type ProcessManager interface {
	ContainerPid(string) (int, error)
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

func NewManager(logger *logrus.Entry, hcsClient HCSClient, mounter Mounter, state StateManager, id, rootDir string, processManager ProcessManager) *Manager {
	return &Manager{
		logger:         logger,
		hcsClient:      hcsClient,
		mounter:        mounter,
		state:          state,
		id:             id,
		rootDir:        rootDir,
		processManager: processManager,
	}
}

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

	if err := m.state.Initialize(bundlePath); err != nil {
		cleanupContainer()
		return nil, err
	}

	return spec, nil
}

func (m *Manager) loadBundle(bundlePath string) (*specs.Spec, error) {
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

func (m *Manager) Delete(force bool) error {
	_, err := m.hcsClient.GetContainerProperties(m.id)
	if err != nil {
		if force {
			_, ok := err.(*hcs.NotFoundError)
			if ok {
				return nil
			}
		}

		return err
	}

	query := hcsshim.ComputeSystemQuery{Owners: []string{m.id}}
	sidecardContainerProperties, err := m.hcsClient.GetContainers(query)
	if err != nil {
		return err
	}
	containerIdsToDelete := []string{}
	for _, sidecardContainerProperty := range sidecardContainerProperties {
		containerIdsToDelete = append(containerIdsToDelete, sidecardContainerProperty.ID)
	}
	containerIdsToDelete = append(containerIdsToDelete, m.id)

	var errors []string
	for _, containerIdToDelete := range containerIdsToDelete {
		pid, err := m.processManager.ContainerPid(containerIdToDelete)
		if err != nil {
			logrus.Error(err.Error())
			errors = append(errors, err.Error())
			continue
		}

		err = m.mounter.Unmount(pid)
		if err != nil {
			logrus.Error(err.Error())
			errors = append(errors, err.Error())
		}

		container, err := m.hcsClient.OpenContainer(containerIdToDelete)
		if err != nil {
			logrus.Error(err.Error())
			errors = append(errors, err.Error())
			continue
		}

		err = m.deleteContainer(containerIdToDelete, container)
		if err != nil {
			logrus.Error(err.Error())
			errors = append(errors, err.Error())
			continue
		}
	}

	if len(errors) == 0 {
		return nil
	} else {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
}

func (m *Manager) State() (*specs.State, error) {
	containerState, err := m.state.Get()
	if _, ok := err.(*state.FileNotFoundError); ok {
		return nil, errors.New(fmt.Sprintf("container not found: %s", m.id))
	}
	if err != nil {
		panic(err)
	}

	return containerState, nil

}

func (m *Manager) Start() error {
	ociState, err := m.State()
	if err != nil {
		return err
	}

	if ociState.Status != "created" {
		return fmt.Errorf("cannot start a container in the %s state", ociState.Status)
	}

	spec, err := m.loadBundle(ociState.Bundle)
	if err != nil {
		return err
	}

	containerState := state.ContainerState{Bundle: ociState.Bundle}
	writeContainerState := func() error {
		return m.state.WriteContainerState(containerState)
	}

	proc, err := m.Exec(spec.Process, false)
	if err != nil {
		containerState.UserProgramExecFailed = true
		writeContainerState()
		return err
	}
	defer proc.Close()

	containerState.UserProgramPID = proc.Pid()

	// trying to open the process to get a handle + its start time should be valid
	// here, since the hcsshim.process struct has an open handle to the process,
	// and the PID will not be reused until all open handles are closed.
	//
	// https://blogs.msdn.microsoft.com/oldnewthing/20110107-00/?p=11803

	containerState.UserProgramStartTime, err = state.ProcessStartTime(uint32(proc.Pid()))
	if err != nil {
		writeContainerState()
		return err
	}

	return writeContainerState()
}

func (m *Manager) Exec(processSpec *specs.Process, createIOPipes bool) (hcsshim.Process, error) {
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
		return nil, &CouldNotCreateProcessError{Id: m.id, Command: command}
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

	stats.Data.Memory.Raw.TotalRss = containerStats.Memory.UsageCommitBytes
	stats.Data.CPUStats.CPUUsage.Usage = containerStats.Processor.TotalRuntime100ns * 100
	stats.Data.CPUStats.CPUUsage.User = containerStats.Processor.RuntimeUser100ns * 100
	stats.Data.CPUStats.CPUUsage.System = containerStats.Processor.RuntimeKernel100ns * 100

	return stats, nil
}

func (m *Manager) deleteContainer(containerId string, container hcs.Container) error {
	props, err := m.hcsClient.GetContainerProperties(containerId)
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

	return os.RemoveAll(filepath.Join(m.rootDir, m.id))
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
