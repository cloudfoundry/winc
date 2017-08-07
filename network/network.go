package network

import (
	"encoding/json"
	"strings"
	"time"

	"code.cloudfoundry.org/winc/hcs"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

const WINC_NETWORK = "winc-nat"
const SUBNET_RANGE = "172.35.0.0/22"
const GATEWAY_ADDRESS = "172.35.0.1"

//go:generate counterfeiter . PortAllocator
type PortAllocator interface {
	AllocatePort(handle string, port int) (int, error)
	ReleaseAllPorts(handle string) error
}

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	GetHNSNetworkByName(string) (*hcsshim.HNSNetwork, error)
	CreateNetwork(*hcsshim.HNSNetwork) (*hcsshim.HNSNetwork, error)
	CreateEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	GetHNSEndpointByID(string) (*hcsshim.HNSEndpoint, error)
	DeleteEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
}

type Manager struct {
	hcsClient     HCSClient
	portAllocator PortAllocator
}

func NewManager(client HCSClient, portAllocator PortAllocator) *Manager {
	return &Manager{
		hcsClient:     client,
		portAllocator: portAllocator,
	}
}

func (n *Manager) AttachEndpointToConfig(config hcsshim.ContainerConfig, containerID string) (hcsshim.ContainerConfig, error) {
	wincNATNetwork, err := n.getWincNATNetwork()
	if err != nil {
		logrus.Error(err.Error())
		return hcsshim.ContainerConfig{}, err
	}

	appPortMapping, err := n.portMapping(8080, containerID)
	if err != nil {
		logrus.Error(err.Error())
		n.cleanupPorts(containerID)
		return hcsshim.ContainerConfig{}, err
	}

	sshPortMapping, err := n.portMapping(2222, containerID)
	if err != nil {
		logrus.Error(err.Error())
		n.cleanupPorts(containerID)
		return hcsshim.ContainerConfig{}, err
	}

	endpoint := &hcsshim.HNSEndpoint{
		Name:           containerID,
		VirtualNetwork: wincNATNetwork.Id,
		Policies:       []json.RawMessage{appPortMapping, sshPortMapping},
	}

	endpointID, err := n.createEndpoint(endpoint)
	if err != nil {
		logrus.Error(err.Error())
		n.cleanupPorts(containerID)
		return hcsshim.ContainerConfig{}, err
	}

	config.EndpointList = []string{endpointID}
	return config, nil
}

func (n *Manager) getWincNATNetwork() (*hcsshim.HNSNetwork, error) {
	var wincNATNetwork *hcsshim.HNSNetwork
	var err error

	for i := 0; i < 10 && wincNATNetwork == nil; i++ {
		time.Sleep(time.Duration(i) * 100 * time.Millisecond)
		wincNATNetwork, err = n.hcsClient.GetHNSNetworkByName(WINC_NETWORK)
		if err != nil && !strings.Contains(err.Error(), "Network "+WINC_NETWORK+" not found") {
			logrus.Error(err.Error())
			return nil, err
		}

		if wincNATNetwork == nil {
			network := &hcsshim.HNSNetwork{
				Name: WINC_NETWORK,
				Type: "nat",
				Subnets: []hcsshim.Subnet{
					{AddressPrefix: SUBNET_RANGE, GatewayAddress: GATEWAY_ADDRESS},
				},
			}
			wincNATNetwork, err = n.hcsClient.CreateNetwork(network)
			if err != nil && !strings.Contains(err.Error(), "HNS failed with error : {Object Exists}") {
				logrus.Error(err.Error())
				return nil, err
			}
		}
	}

	if wincNATNetwork == nil {
		return nil, &NoNATNetworkError{Name: WINC_NETWORK}
	}

	return wincNATNetwork, nil
}

func (n *Manager) portMapping(containerPort int, containerID string) (json.RawMessage, error) {
	hostPort, err := n.portAllocator.AllocatePort(containerID, 0)
	if err != nil {
		return nil, err
	}
	portMapping := hcsshim.NatPolicy{
		Type:         "NAT",
		Protocol:     "TCP",
		InternalPort: uint16(containerPort),
		ExternalPort: uint16(hostPort),
	}

	portMappingJSON, err := json.Marshal(portMapping)
	if err != nil {
		return nil, err
	}

	return portMappingJSON, nil
}

func (n *Manager) createEndpoint(endpoint *hcsshim.HNSEndpoint) (string, error) {
	var createErr error
	var createdEndpoint *hcsshim.HNSEndpoint
	for i := 0; i < 3 && createdEndpoint == nil; i++ {
		createdEndpoint, createErr = n.hcsClient.CreateEndpoint(endpoint)
		if createErr != nil {
			if createErr.Error() != "HNS failed with error : Unspecified error" {
				return "", createErr
			}
			logrus.Error(createErr.Error())
		}
	}

	if createdEndpoint == nil {
		return "", createErr
	}

	return createdEndpoint.Id, nil
}

func (n *Manager) DeleteContainerEndpoints(container hcs.Container, containerID string) error {
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

func (n *Manager) DeleteEndpointsById(ids []string, containerID string) error {
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

func (n *Manager) cleanupPorts(containerID string) {
	releaseErr := n.portAllocator.ReleaseAllPorts(containerID)
	if releaseErr != nil {
		logrus.Error(releaseErr.Error())
	}
}
