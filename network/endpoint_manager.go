package network

import (
	"encoding/json"

	"code.cloudfoundry.org/winc/hcs"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

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
	wincNATNetwork, err := e.hcsClient.GetHNSNetworkByName(e.networkName)
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
