package hcs

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/winc/filelock"
	"code.cloudfoundry.org/winc/network/netinterface"
	"github.com/Microsoft/hcsshim"
)

func NewClient() *Client {
	return &Client{
		layerCreateLock: filelock.NewLocker("C:\\var\\vcap\\data\\winc-image\\create.lock"),
	}
}

type Client struct {
	layerCreateLock filelock.FileLocker
}

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

func (c *Client) UpdateEndpoint(endpoint *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	return endpoint.Update()
}

func (c *Client) DeleteEndpoint(endpoint *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	return endpoint.Delete()
}

func (c *Client) CreateNetwork(network *hcsshim.HNSNetwork) (*hcsshim.HNSNetwork, error) {
	net, err := network.Create()
	if err != nil {
		return nil, err
	}

	interfaceUp := false
	alias := fmt.Sprintf("vEthernet (%s)", net.Name)

	for i := 0; i < 10; i++ {
		time.Sleep(200 * time.Duration(i) * time.Millisecond)
		interfaceUp, err = netinterface.InterfaceExists(alias)
		if err != nil {
			return nil, err
		}

		if interfaceUp {
			break
		}

	}

	if !interfaceUp {
		return nil, fmt.Errorf("interface %s not created in time", alias)
	}

	return net, nil
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

func (c *Client) GetHNSEndpointByName(name string) (*hcsshim.HNSEndpoint, error) {
	return hcsshim.GetHNSEndpointByName(name)
}

func (c *Client) GetHNSNetworkByName(name string) (*hcsshim.HNSNetwork, error) {
	return hcsshim.GetHNSNetworkByName(name)
}

func (c *Client) HotAttachEndpoint(containerID string, endpointID string) error {
	if err := hcsshim.HotAttachEndpoint(containerID, endpointID); err != nil {
		return err
	}

	containerInterfaceUp := false
	alias := fmt.Sprintf("vEthernet (%s)", containerID)

	for i := 0; i < 10; i++ {
		var err error
		containerInterfaceUp, err = netinterface.InterfaceExists(alias)
		if err != nil {
			return err
		}

		if containerInterfaceUp {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	if !containerInterfaceUp {
		return fmt.Errorf("container interface %s not created in time", alias)
	}

	return nil
}

func (c *Client) HotDetachEndpoint(containerID string, endpointID string) error {
	return hcsshim.HotDetachEndpoint(containerID, endpointID)
}

func (c *Client) CreateLayer(di hcsshim.DriverInfo, id string, parentId string, parentLayerPaths []string) error {
	f, err := c.layerCreateLock.Open()
	if err != nil {
		return err
	}
	defer f.Close()

	if err := hcsshim.CreateSandboxLayer(di, id, parentId, parentLayerPaths); err != nil {
		return err
	}

	if err := hcsshim.ActivateLayer(di, id); err != nil {
		return err
	}

	return hcsshim.PrepareLayer(di, id, parentLayerPaths)
}

func (c *Client) RemoveLayer(di hcsshim.DriverInfo, id string) error {
	var unprepareErr, deactivateErr, destroyErr error

	for i := 0; i < 3; i++ {
		unprepareErr = hcsshim.UnprepareLayer(di, id)
		deactivateErr = hcsshim.DeactivateLayer(di, id)
		destroyErr = hcsshim.DestroyLayer(di, id)
		if destroyErr == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to remove layer (unprepare error: %s, deactivate error: %s, destroy error: %s)", unprepareErr.Error(), deactivateErr.Error(), destroyErr.Error())
}
