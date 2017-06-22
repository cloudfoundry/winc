package hcsclient

import (
	"io"
	"time"

	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter . Client
type Client interface {
	GetContainers(q hcsshim.ComputeSystemQuery) ([]hcsshim.ContainerProperties, error)
	NameToGuid(name string) (hcsshim.GUID, error)
	GetLayerMountPath(info hcsshim.DriverInfo, id string) (string, error)
	CreateContainer(id string, config *hcsshim.ContainerConfig) (hcsshim.Container, error)
	OpenContainer(id string) (hcsshim.Container, error)
	IsPending(err error) bool
	CreateSandboxLayer(info hcsshim.DriverInfo, layerId, parentId string, parentLayerPaths []string) error
	ActivateLayer(info hcsshim.DriverInfo, id string) error
	PrepareLayer(info hcsshim.DriverInfo, layerId string, parentLayerPaths []string) error
	UnprepareLayer(info hcsshim.DriverInfo, layerId string) error
	DeactivateLayer(info hcsshim.DriverInfo, id string) error
	DestroyLayer(info hcsshim.DriverInfo, id string) error
	GetContainerProperties(id string) (hcsshim.ContainerProperties, error)
	GetHNSNetworkByName(networkName string) (*hcsshim.HNSNetwork, error)
	GetHNSEndpointByID(id string) (*hcsshim.HNSEndpoint, error)
	CreateEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	DeleteEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
}

//go:generate counterfeiter . Container
type Container interface {
	Start() error
	Shutdown() error
	Terminate() error
	Wait() error
	WaitTimeout(time.Duration) error
	Pause() error
	Resume() error
	HasPendingUpdates() (bool, error)
	Statistics() (hcsshim.Statistics, error)
	ProcessList() ([]hcsshim.ProcessListItem, error)
	CreateProcess(c *hcsshim.ProcessConfig) (hcsshim.Process, error)
	OpenProcess(pid int) (hcsshim.Process, error)
	Close() error
	Modify(config *hcsshim.ResourceModificationRequestResponse) error
}

//go:generate counterfeiter . Process
type Process interface {
	Pid() int
	Kill() error
	Wait() error
	WaitTimeout(time.Duration) error
	ExitCode() (int, error)
	ResizeConsole(width, height uint16) error
	Stdio() (io.WriteCloser, io.ReadCloser, io.ReadCloser, error)
	CloseStdin() error
	Close() error
}

type HCSClient struct{}

func (c *HCSClient) GetContainers(q hcsshim.ComputeSystemQuery) ([]hcsshim.ContainerProperties, error) {
	return hcsshim.GetContainers(q)
}

func (c *HCSClient) NameToGuid(name string) (hcsshim.GUID, error) {
	return hcsshim.NameToGuid(name)
}

func (c *HCSClient) GetLayerMountPath(info hcsshim.DriverInfo, id string) (string, error) {
	return hcsshim.GetLayerMountPath(info, id)
}

func (c *HCSClient) CreateContainer(id string, config *hcsshim.ContainerConfig) (hcsshim.Container, error) {
	return hcsshim.CreateContainer(id, config)
}

func (c *HCSClient) OpenContainer(id string) (hcsshim.Container, error) {
	return hcsshim.OpenContainer(id)
}

func (c *HCSClient) IsPending(err error) bool {
	return hcsshim.IsPending(err)
}

func (c *HCSClient) CreateSandboxLayer(info hcsshim.DriverInfo, layerId, parentId string, parentLayerPaths []string) error {
	return hcsshim.CreateSandboxLayer(info, layerId, parentId, parentLayerPaths)
}

func (c *HCSClient) ActivateLayer(info hcsshim.DriverInfo, id string) error {
	return hcsshim.ActivateLayer(info, id)
}

func (c *HCSClient) PrepareLayer(info hcsshim.DriverInfo, layerId string, parentLayerPaths []string) error {
	return hcsshim.PrepareLayer(info, layerId, parentLayerPaths)
}

func (c *HCSClient) UnprepareLayer(info hcsshim.DriverInfo, layerId string) error {
	return hcsshim.UnprepareLayer(info, layerId)
}

func (c *HCSClient) DeactivateLayer(info hcsshim.DriverInfo, id string) error {
	return hcsshim.DeactivateLayer(info, id)
}

func (c *HCSClient) DestroyLayer(info hcsshim.DriverInfo, id string) error {
	return hcsshim.DestroyLayer(info, id)
}

func (c *HCSClient) GetContainerProperties(id string) (hcsshim.ContainerProperties, error) {
	query := hcsshim.ComputeSystemQuery{
		IDs:    []string{id},
		Owners: []string{"winc"},
	}
	cps, err := c.GetContainers(query)
	if err != nil {
		return hcsshim.ContainerProperties{}, err
	}

	if len(cps) == 0 {
		return hcsshim.ContainerProperties{}, &NotFoundError{Id: id}
	}

	if len(cps) > 1 {
		return hcsshim.ContainerProperties{}, &DuplicateError{Id: id}
	}

	return cps[0], nil
}

func (c *HCSClient) CreateEndpoint(endpoint *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	return endpoint.Create()
}

func (c *HCSClient) DeleteEndpoint(endpoint *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	return endpoint.Delete()
}

func (c *HCSClient) GetHNSNetworkByName(networkName string) (*hcsshim.HNSNetwork, error) {
	return hcsshim.GetHNSNetworkByName(networkName)
}

func (c *HCSClient) GetHNSEndpointByID(id string) (*hcsshim.HNSEndpoint, error) {
	return hcsshim.GetHNSEndpointByID(id)
}
