package endpoint

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.cloudfoundry.org/winc/network"
	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	GetHNSNetworkByName(string) (*hcsshim.HNSNetwork, error)
	CreateEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	GetHNSEndpointByID(string) (*hcsshim.HNSEndpoint, error)
	GetHNSEndpointByName(string) (*hcsshim.HNSEndpoint, error)
	DeleteEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	HotAttachEndpoint(containerID string, endpointID string) error
	HotDetachEndpoint(containerID string, endpointID string) error
}

type EndpointManager struct {
	hcsClient   HCSClient
	containerId string
	config      network.Config
}

func NewEndpointManager(hcsClient HCSClient, containerId string, config network.Config) *EndpointManager {
	return &EndpointManager{
		hcsClient:   hcsClient,
		containerId: containerId,
		config:      config,
	}
}

func (e *EndpointManager) Create(natPolicies []hcsshim.NatPolicy) (hcsshim.HNSEndpoint, error) {
	network, err := e.hcsClient.GetHNSNetworkByName(e.config.NetworkName)
	if err != nil {
		return hcsshim.HNSEndpoint{}, err
	}

	policies := []json.RawMessage{}
	for _, natPolicy := range natPolicies {
		data, err := json.Marshal(natPolicy)
		if err != nil {
			return hcsshim.HNSEndpoint{}, err
		}
		policies = append(policies, data)
	}

	endpoint := &hcsshim.HNSEndpoint{
		VirtualNetwork: network.Id,
		Name:           e.containerId,
		Policies:       policies,
	}

	if len(e.config.DNSServers) > 0 {
		endpoint.DNSServerList = strings.Join(e.config.DNSServers, ",")
	}

	createdEndpoint, err := e.createEndpoint(endpoint)
	if err != nil {
		return hcsshim.HNSEndpoint{}, err
	}

	if err := e.hcsClient.HotAttachEndpoint(e.containerId, createdEndpoint.Id); err != nil {
		logrus.Error(fmt.Sprintf("Unable to attach endpoint %s to container %s: %s", createdEndpoint.Id, e.containerId, err.Error()))

		if _, err := e.hcsClient.DeleteEndpoint(createdEndpoint); err != nil {
			logrus.Error(fmt.Sprintf("Error deleting endpoint %s: %s", createdEndpoint.Id, err.Error()))
		}

		return hcsshim.HNSEndpoint{}, err
	}

	return *createdEndpoint, nil
}

func (e *EndpointManager) Delete() error {
	endpoint, err := e.hcsClient.GetHNSEndpointByName(e.containerId)
	if err != nil {
		if err.Error() == fmt.Sprintf("Endpoint %s not found", e.containerId) {
			return nil
		}

		return err
	}

	var detachErr error
	err = e.hcsClient.HotDetachEndpoint(e.containerId, endpoint.Id)
	if err != hcsshim.ErrComputeSystemDoesNotExist {
		detachErr = err
	}

	_, deleteErr := e.hcsClient.DeleteEndpoint(endpoint)

	if detachErr != nil && deleteErr != nil {
		return fmt.Errorf("%s, %s", detachErr.Error(), deleteErr.Error())
	}

	if detachErr != nil {
		return detachErr
	}

	if deleteErr != nil {
		return deleteErr
	}

	return nil
}

func (e *EndpointManager) createEndpoint(endpoint *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	var createErr error
	var createdEndpoint *hcsshim.HNSEndpoint
	for i := 0; i < 3 && createdEndpoint == nil; i++ {
		createdEndpoint, createErr = e.hcsClient.CreateEndpoint(endpoint)
		if createErr != nil {
			if createErr.Error() != "HNS failed with error : Unspecified error" {
				return nil, createErr
			}
		}
	}

	if createdEndpoint == nil {
		return nil, createErr
	}

	return createdEndpoint, nil
}
