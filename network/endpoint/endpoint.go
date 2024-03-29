package endpoint

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/firewall"
	"code.cloudfoundry.org/winc/network/netinterface"
	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

//go:generate counterfeiter -o fakes/hcs_client.go --fake-name HCSClient . HCSClient
type HCSClient interface {
	GetHNSNetworkByName(string) (*hcsshim.HNSNetwork, error)
	CreateEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	UpdateEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	GetHNSEndpointByID(string) (*hcsshim.HNSEndpoint, error)
	GetHNSEndpointByName(string) (*hcsshim.HNSEndpoint, error)
	DeleteEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	HotAttachEndpoint(containerID string, endpointID string, endpointReady func() (bool, error)) error
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

func (e *EndpointManager) Create() (hcsshim.HNSEndpoint, error) {
	network, err := e.hcsClient.GetHNSNetworkByName(e.config.NetworkName)
	if err != nil {
		return hcsshim.HNSEndpoint{}, err
	}

	endpoint := &hcsshim.HNSEndpoint{
		VirtualNetwork: network.Id,
		Name:           e.containerId,
	}

	if e.config.MaximumOutgoingBandwidth != 0 {
		policy, err := json.Marshal(hcsshim.QosPolicy{
			Type:                            hcsshim.QOS,
			MaximumOutgoingBandwidthInBytes: uint64(e.config.MaximumOutgoingBandwidth),
		})
		if err != nil {
			return hcsshim.HNSEndpoint{}, err
		}

		endpoint.Policies = []json.RawMessage{policy}
	}

	if len(e.config.DNSServers) > 0 {
		endpoint.DNSServerList = strings.Join(e.config.DNSServers, ",")
	}

	createdEndpoint, err := e.createEndpoint(endpoint)
	if err != nil {
		return hcsshim.HNSEndpoint{}, err
	}

	attachedEndpoint, err := e.attachEndpoint(createdEndpoint)
	if err != nil {
		if _, err := e.hcsClient.DeleteEndpoint(createdEndpoint); err != nil {
			logrus.Error(fmt.Sprintf("Error deleting endpoint %s: %s", endpoint.Id, err.Error()))
		}

		return hcsshim.HNSEndpoint{}, err
	}

	return *attachedEndpoint, nil
}

func (e *EndpointManager) attachEndpoint(endpoint *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	endpointReady := func() (bool, error) {
		interfaceAlias := fmt.Sprintf("vEthernet (%s)", e.containerId)
		return netinterface.InterfaceExists(interfaceAlias)
	}

	if err := e.hcsClient.HotAttachEndpoint(e.containerId, endpoint.Id, endpointReady); err != nil {
		return nil, err
	}

	allocatedEndpoint, err := e.hcsClient.GetHNSEndpointByID(endpoint.Id)
	if err != nil {
		logrus.Error(fmt.Sprintf("Unable to load updated endpoint %s: %s", endpoint.Id, err.Error()))
		return nil, err
	}

	return allocatedEndpoint, nil
}

func (e *EndpointManager) ApplyPolicies(endpoint hcsshim.HNSEndpoint, nats []*hcsshim.NatPolicy, acls []*hcsshim.ACLPolicy) (hcsshim.HNSEndpoint, error) {
	var policies []json.RawMessage

	if len(acls) == 0 {
		// make sure everything's blocked if no netout rules present
		acls = []*hcsshim.ACLPolicy{
			{
				Type:      hcsshim.ACL,
				Action:    hcsshim.Block,
				Direction: hcsshim.Out,
				Protocol:  uint16(firewall.NET_FW_IP_PROTOCOL_ANY),
			},
			{
				Type:      hcsshim.ACL,
				Action:    hcsshim.Block,
				Direction: hcsshim.In,
				Protocol:  uint16(firewall.NET_FW_IP_PROTOCOL_ANY),
			},
		}
	}

	for _, acl := range acls {
		policy, err := json.Marshal(acl)
		if err != nil {
			return hcsshim.HNSEndpoint{}, err
		}
		policies = append(policies, policy)
	}

	for _, nat := range nats {
		policy, err := json.Marshal(nat)
		if err != nil {
			return hcsshim.HNSEndpoint{}, err
		}
		policies = append(policies, policy)
	}

	endpoint.Policies = append(endpoint.Policies, policies...)

	updatedEndpoint, err := e.hcsClient.UpdateEndpoint(&endpoint)
	if err != nil {
		return hcsshim.HNSEndpoint{}, err
	}

	return *updatedEndpoint, nil
}

func (e *EndpointManager) Delete() error {
	endpoint, err := e.hcsClient.GetHNSEndpointByName(e.containerId)
	if err != nil {
		if _, ok := err.(hcsshim.EndpointNotFoundError); ok {
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
