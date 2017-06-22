package network

import (
	"encoding/json"

	"code.cloudfoundry.org/winc/hcsclient"
	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

//go:generate counterfeiter . NetworkManager
type NetworkManager interface {
	AttachEndpointToConfig(config hcsshim.ContainerConfig, containerID string) (hcsshim.ContainerConfig, error)
	DeleteContainerEndpoints(container hcsclient.Container, containerID string) error
	DeleteEndpointsById(ids []string, containerID string) error
}

//go:generate counterfeiter . PortAllocator
type PortAllocator interface {
	AllocatePort(handle string, port int) (int, error)
	ReleaseAllPorts(handle string) error
}

type networkManager struct {
	hcsClient     hcsclient.Client
	portAllocator PortAllocator
}

func NewNetworkManager(client hcsclient.Client, portAllocator PortAllocator) NetworkManager {
	return &networkManager{
		hcsClient:     client,
		portAllocator: portAllocator,
	}
}

func (n *networkManager) AttachEndpointToConfig(config hcsshim.ContainerConfig, containerID string) (hcsshim.ContainerConfig, error) {
	hostPort, err := n.portAllocator.AllocatePort(containerID, 0)
	if err != nil {
		return hcsshim.ContainerConfig{}, err
	}

	network, err := n.hcsClient.GetHNSNetworkByName("nat")
	if err != nil {
		n.cleanupPorts(containerID)
		return hcsshim.ContainerConfig{}, err
	}

	portMapping := hcsshim.NatPolicy{
		Type:         "NAT",
		Protocol:     "TCP",
		InternalPort: 8080,
		ExternalPort: uint16(hostPort),
	}

	portMappingJSON, err := json.Marshal(portMapping)
	if err != nil {
		n.cleanupPorts(containerID)
		return hcsshim.ContainerConfig{}, err
	}

	endpoint := &hcsshim.HNSEndpoint{
		Name:           containerID,
		VirtualNetwork: network.Id,
		Policies:       []json.RawMessage{portMappingJSON},
	}

	endpoint, err = n.hcsClient.CreateEndpoint(endpoint)
	if err != nil {
		n.cleanupPorts(containerID)
		return hcsshim.ContainerConfig{}, err
	}

	config.EndpointList = []string{endpoint.Id}
	return config, nil
}

func (n *networkManager) DeleteContainerEndpoints(container hcsclient.Container, containerID string) error {
	stats, err := container.Statistics()
	if err != nil {
		return err
	}

	var endpointIDs []string
	for _, network := range stats.Network {
		endpointIDs = append(endpointIDs, network.EndpointId)
	}

	return n.DeleteEndpointsById(endpointIDs, containerID)
}

func (n *networkManager) DeleteEndpointsById(ids []string, containerID string) error {
	var deleteErrors []error
	for _, endpointId := range ids {
		endpoint, err := n.hcsClient.GetHNSEndpointByID(endpointId)
		if err != nil {
			logrus.Error(err.Error())
			continue
		}

		_, deleteErr := n.hcsClient.DeleteEndpoint(endpoint)
		if deleteErr != nil {
			deleteErrors = append(deleteErrors, deleteErr)
		}
	}

	if len(deleteErrors) > 0 {
		for _, deleteErr := range deleteErrors {
			logrus.Error(deleteErr.Error())
		}

		return deleteErrors[0]
	}

	return n.portAllocator.ReleaseAllPorts(containerID)
}

func (n *networkManager) cleanupPorts(containerID string) {
	releaseErr := n.portAllocator.ReleaseAllPorts(containerID)
	if releaseErr != nil {
		logrus.Error(releaseErr.Error())
	}
}
