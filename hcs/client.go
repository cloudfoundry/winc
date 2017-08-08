package hcs

import (
	"strings"

	"github.com/Microsoft/hcsshim"
)

type Client struct{}

func (c *Client) GetContainers(q hcsshim.ComputeSystemQuery) ([]hcsshim.ContainerProperties, error) {
	return hcsshim.GetContainers(q)
}

func (c *Client) NameToGuid(name string) (hcsshim.GUID, error) {
	return hcsshim.NameToGuid(name)
}

func (c *Client) GetLayerMountPath(info hcsshim.DriverInfo, id string) (string, error) {
	return hcsshim.GetLayerMountPath(info, id)
}

func (c *Client) CreateContainer(id string, config *hcsshim.ContainerConfig) (Container, error) {
	return hcsshim.CreateContainer(id, config)
}

func (c *Client) OpenContainer(id string) (Container, error) {
	return hcsshim.OpenContainer(id)
}

func (c *Client) IsPending(err error) bool {
	return hcsshim.IsPending(err)
}

func (c *Client) CreateSandboxLayer(info hcsshim.DriverInfo, layerId, parentId string, parentLayerPaths []string) error {
	return hcsshim.CreateSandboxLayer(info, layerId, parentId, parentLayerPaths)
}

func (c *Client) ActivateLayer(info hcsshim.DriverInfo, id string) error {
	return hcsshim.ActivateLayer(info, id)
}

func (c *Client) PrepareLayer(info hcsshim.DriverInfo, layerId string, parentLayerPaths []string) error {
	return hcsshim.PrepareLayer(info, layerId, parentLayerPaths)
}

func (c *Client) DestroyLayer(info hcsshim.DriverInfo, id string) error {
	if err := hcsshim.UnprepareLayer(info, id); !shouldContinueDestroyingLayer(err) {
		return err
	}

	if err := hcsshim.DeactivateLayer(info, id); err != nil {
		return err
	}

	return hcsshim.DestroyLayer(info, id)
}

func (c *Client) LayerExists(info hcsshim.DriverInfo, id string) (bool, error) {
	return hcsshim.LayerExists(info, id)
}

func (c *Client) GetContainerProperties(id string) (hcsshim.ContainerProperties, error) {
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

func (c *Client) CreateEndpoint(endpoint *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	return endpoint.Create()
}

func (c *Client) DeleteEndpoint(endpoint *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	return endpoint.Delete()
}

func (c *Client) CreateNetwork(network *hcsshim.HNSNetwork) (*hcsshim.HNSNetwork, error) {
	return network.Create()
}

func (c *Client) DeleteNetwork(network *hcsshim.HNSNetwork) (*hcsshim.HNSNetwork, error) {
	return network.Delete()
}

func (c *Client) HNSListNetworkRequest() ([]hcsshim.HNSNetwork, error) {
	return hcsshim.HNSListNetworkRequest("GET", "", "")
}

func (c *Client) GetHNSEndpointByID(id string) (*hcsshim.HNSEndpoint, error) {
	return hcsshim.GetHNSEndpointByID(id)
}

func (c *Client) GetHNSNetworkByName(name string) (*hcsshim.HNSNetwork, error) {
	return hcsshim.GetHNSNetworkByName(name)
}

func (c *Client) Retryable(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "This operation returned because the timeout period expired"))
}

func shouldContinueDestroyingLayer(err error) bool {
	return err == nil ||
		strings.Contains(err.Error(), "hcsshim::UnprepareLayer failed in Win32: The system could not find the instance specified") ||
		strings.Contains(err.Error(), "hcsshim::UnprepareLayer failed in Win32: The specified network resource or device is no longer available")
}
