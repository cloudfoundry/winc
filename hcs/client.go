package hcs

import (
	"fmt"
	"time"

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

func (c *Client) GetContainerProperties(id string) (hcsshim.ContainerProperties, error) {
	query := hcsshim.ComputeSystemQuery{
		IDs: []string{id},
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

func (c *Client) CreateNetwork(network *hcsshim.HNSNetwork, networkReady func() (bool, error)) (*hcsshim.HNSNetwork, error) {
	var net *hcsshim.HNSNetwork
	var err error
	/*
	* This @errElmNotFound error is notorious for being thrown sometimes without any real reason
	* (at least we believe so) -- possibly a bug in the Windows container networking stack.
	* Let's give it a 2nd chance (and a 3rd) to get it right!
	 */
	const errElmNotFound = "network create: HNS failed with error : Element not found. "

	for i := 0; i < 3 && net == nil; i++ {
		net, err = network.Create()
		if err != nil && err.Error() != errElmNotFound {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}

	networkUp := false

	for i := 0; i < 10; i++ {
		time.Sleep(200 * time.Duration(i) * time.Millisecond)
		networkUp, err = networkReady()
		if err != nil {
			return nil, err
		}

		if networkUp {
			break
		}

	}

	if !networkUp {
		return nil, fmt.Errorf("network %s not ready in time", net.Name)
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

func (c *Client) HotAttachEndpoint(containerID string, endpointID string, endpointReady func() (bool, error)) error {
	if err := hcsshim.HotAttachEndpoint(containerID, endpointID); err != nil {
		return err
	}

	endpointUp := false

	for i := 0; i < 10; i++ {
		var err error
		endpointUp, err = endpointReady()
		if err != nil {
			return err
		}

		if endpointUp {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	if !endpointUp {
		return fmt.Errorf("endpoint %s not ready in time", endpointID)
	}

	return nil
}

func (c *Client) HotDetachEndpoint(containerID string, endpointID string) error {
	return hcsshim.HotDetachEndpoint(containerID, endpointID)
}
