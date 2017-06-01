package container

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/winc/hcsclient"
	"code.cloudfoundry.org/winc/sandbox"
	"github.com/Microsoft/hcsshim"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const destroyTimeout = time.Second

type ContainerManager interface {
	Create(spec *specs.Spec) error
	Delete() error
	State() (*specs.State, error)
	Exec(*specs.Process) (hcsshim.Process, error)
}

type containerManager struct {
	hcsClient      hcsclient.Client
	sandboxManager sandbox.SandboxManager
	id             string
}

func NewManager(hcsClient hcsclient.Client, sandboxManager sandbox.SandboxManager, containerId string) ContainerManager {
	return &containerManager{
		hcsClient:      hcsClient,
		sandboxManager: sandboxManager,
		id:             containerId,
	}
}

func (c *containerManager) Create(spec *specs.Spec) error {
	_, err := c.hcsClient.GetContainerProperties(c.id)
	if err == nil {
		return &hcsclient.AlreadyExistsError{Id: c.id}
	}
	if _, ok := err.(*hcsclient.NotFoundError); !ok {
		return err
	}

	bundlePath := c.sandboxManager.BundlePath()
	if filepath.Base(bundlePath) != c.id {
		return &hcsclient.InvalidIdError{Id: c.id}
	}

	if err := c.sandboxManager.Create(spec.Root.Path); err != nil {
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
		layerGuid, err := c.hcsClient.NameToGuid(layerId)
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
	volumePath, err := c.hcsClient.GetLayerMountPath(driverInfo, c.id)
	if err != nil {
		return err
	} else if volumePath == "" {
		return &hcsclient.MissingVolumePathError{Id: c.id}
	}

	mappedDirs := []hcsshim.MappedDir{}
	for _, d := range spec.Mounts {
		mappedDirs = append(mappedDirs, hcsshim.MappedDir{
			HostPath:      d.Source,
			ContainerPath: d.Destination,
			ReadOnly:      true,
		})
	}

	containerConfig := &hcsshim.ContainerConfig{
		SystemType:        "Container",
		Name:              bundlePath,
		VolumePath:        volumePath,
		Owner:             "winc",
		LayerFolderPath:   bundlePath,
		Layers:            layerInfos,
		MappedDirectories: mappedDirs,
	}

	container, err := c.hcsClient.CreateContainer(c.id, containerConfig)
	if err != nil {
		_ = c.sandboxManager.Delete()
		return err
	}

	if err := container.Start(); err != nil {
		_ = c.terminateContainer(container)
		return err
	}

	mountPath := filepath.Join(bundlePath, "mnt")
	if err := c.sandboxManager.Mount(mountPath); err != nil {
		_ = c.terminateContainer(container)
		return err
	}

	return nil
}

func (c *containerManager) Delete() error {
	container, err := c.hcsClient.OpenContainer(c.id)
	if err != nil {
		return err
	}

	return c.terminateContainer(container)
}

func (c *containerManager) State() (*specs.State, error) {
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
		Bundle:  c.sandboxManager.BundlePath(),
		Pid:     pid,
	}, nil
}

func (c *containerManager) Exec(processSpec *specs.Process) (hcsshim.Process, error) {
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
		CommandLine:      strings.Join(processSpec.Args, " "),
		CreateStdOutPipe: true,
		CreateStdErrPipe: true,
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
		return nil, &hcsclient.CouldNotCreateProcessError{Id: c.id, Command: command}
	}

	return p, nil
}

func (c *containerManager) containerPid(id string) (int, error) {
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

func (c *containerManager) terminateContainer(container hcsshim.Container) error {
	err := container.Terminate()
	if c.hcsClient.IsPending(err) {
		err = container.WaitTimeout(destroyTimeout)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if err := c.sandboxManager.Delete(); err != nil {
		return err
	}

	return nil
}
