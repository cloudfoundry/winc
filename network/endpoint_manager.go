package network

import (
	"encoding/json"
	"strings"
	"time"

	"code.cloudfoundry.org/winc/hcs"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

const SUBNET_RANGE = "172.30.0.0/22"
const GATEWAY_ADDRESS = "172.30.0.1"

//go:generate counterfeiter . PortAllocator
type PortAllocator interface {
	AllocatePort(handle string, port int) (int, error)
	ReleaseAllPorts(handle string) error
}

type EndpointManager struct {
	hcsClient     HCSClient
	portAllocator PortAllocator
	containerId   string
	networkName   string
}

func NewEndpointManager(hcsClient HCSClient, portAllocator PortAllocator, containerId string, networkName string) *EndpointManager {
	return &EndpointManager{
		hcsClient:     hcsClient,
		portAllocator: portAllocator,
		containerId:   containerId,
		networkName:   networkName,
	}
}

func (e *EndpointManager) AttachEndpointToConfig(containerConfig hcsshim.ContainerConfig) (hcsshim.ContainerConfig, error) {
	wincNATNetwork, err := e.getWincNATNetwork()
	if err != nil {
		logrus.Error(err.Error())
		return hcsshim.ContainerConfig{}, err
	}

	appPortMapping, err := e.portMapping(8080)
	if err != nil {
		logrus.Error(err.Error())
		e.cleanupPorts()
		return hcsshim.ContainerConfig{}, err
	}

	sshPortMapping, err := e.portMapping(2222)
	if err != nil {
		logrus.Error(err.Error())
		e.cleanupPorts()
		return hcsshim.ContainerConfig{}, err
	}

	endpoint := &hcsshim.HNSEndpoint{
		Name:           e.containerId,
		VirtualNetwork: wincNATNetwork.Id,
		Policies:       []json.RawMessage{appPortMapping, sshPortMapping},
	}

	endpointID, err := e.createEndpoint(endpoint)
	if err != nil {
		logrus.Error(err.Error())
		e.cleanupPorts()
		return hcsshim.ContainerConfig{}, err
	}

	containerConfig.EndpointList = []string{endpointID}
	return containerConfig, nil
}

func (e *EndpointManager) getWincNATNetwork() (*hcsshim.HNSNetwork, error) {
	var wincNATNetwork *hcsshim.HNSNetwork
	var err error

	for i := 0; i < 10 && wincNATNetwork == nil; i++ {
		time.Sleep(time.Duration(i) * 100 * time.Millisecond)
		wincNATNetwork, err = e.hcsClient.GetHNSNetworkByName(e.networkName)
		if err != nil && !strings.Contains(err.Error(), "Network "+e.networkName+" not found") {
			logrus.Error(err.Error())
			return nil, err
		}

		if wincNATNetwork == nil {
			network := &hcsshim.HNSNetwork{
				Name: e.networkName,
				Type: "nat",
				Subnets: []hcsshim.Subnet{
					{AddressPrefix: SUBNET_RANGE, GatewayAddress: GATEWAY_ADDRESS},
				},
			}
			wincNATNetwork, err = e.hcsClient.CreateNetwork(network)
			if err != nil && !strings.Contains(err.Error(), "HNS failed with error : {Object Exists}") {
				logrus.Error(err.Error())
				return nil, err
			}
		}
	}

	if wincNATNetwork == nil {
		return nil, &NoNATNetworkError{Name: e.networkName}
	}

	return wincNATNetwork, nil
}

func (e *EndpointManager) portMapping(containerPort int) (json.RawMessage, error) {
	hostPort, err := e.portAllocator.AllocatePort(e.containerId, 0)
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

func (e *EndpointManager) createEndpoint(endpoint *hcsshim.HNSEndpoint) (string, error) {
	var createErr error
	var createdEndpoint *hcsshim.HNSEndpoint
	for i := 0; i < 3 && createdEndpoint == nil; i++ {
		createdEndpoint, createErr = e.hcsClient.CreateEndpoint(endpoint)
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

func (e *EndpointManager) DeleteContainerEndpoints(container hcs.Container) error {
	stats, err := container.Statistics()
	if err != nil {
		return err
	}

	var endpointIDs []string
	for _, network := range stats.Network {
		endpointIDs = append(endpointIDs, network.EndpointId)
	}

	return e.DeleteEndpointsById(endpointIDs)
}

func (e *EndpointManager) DeleteEndpointsById(ids []string) error {
	var deleteErrors []error
	for _, endpointId := range ids {
		endpoint, err := e.hcsClient.GetHNSEndpointByID(endpointId)
		if err != nil {
			logrus.Error(err.Error())
			continue
		}

		_, deleteErr := e.hcsClient.DeleteEndpoint(endpoint)
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

	return e.portAllocator.ReleaseAllPorts(e.containerId)
}

func (e *EndpointManager) cleanupPorts() {
	releaseErr := e.portAllocator.ReleaseAllPorts(e.containerId)
	if releaseErr != nil {
		logrus.Error(releaseErr.Error())
	}
}
