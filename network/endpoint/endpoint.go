package endpoint

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/netinterface"
	"code.cloudfoundry.org/winc/network/netrules"
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

//go:generate counterfeiter -o fakes/netsh_runner.go --fake-name NetShRunner . NetShRunner
type NetShRunner interface {
	RunHost([]string) ([]byte, error)
}

type EndpointManager struct {
	hcsClient   HCSClient
	netsh       NetShRunner
	containerId string
	config      network.Config
}

func NewEndpointManager(hcsClient HCSClient, netsh NetShRunner, containerId string, config network.Config) *EndpointManager {
	return &EndpointManager{
		hcsClient:   hcsClient,
		netsh:       netsh,
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
			Type: hcsshim.QOS,
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

	var compartmentId uint32
	var endpointPortGuid string

	for _, a := range allocatedEndpoint.Resources.Allocators {
		if a.Type == hcsshim.EndpointPortType {
			compartmentId = a.CompartmentId
			endpointPortGuid = a.EndpointPortGuid
			break
		}
	}

	if compartmentId == 0 || endpointPortGuid == "" {
		return nil, fmt.Errorf("invalid endpoint %s allocators: %+v", endpoint.Id, allocatedEndpoint.Resources.Allocators)
	}

	ruleName := fmt.Sprintf("Compartment %d - %s", compartmentId, endpointPortGuid)
	if err := e.deleteFirewallRule(ruleName); err != nil {
		return nil, err
	}

	return allocatedEndpoint, nil

}

func (e *EndpointManager) deleteFirewallRule(ruleName string) error {
	removeFirewallRule := []string{"advfirewall", "firewall", "delete", "rule", fmt.Sprintf(`name=%s`, ruleName)}

	deleted := false
	var err error

	for i := 0; i < 3 && !deleted; i++ {
		if _, err = e.netsh.RunHost(removeFirewallRule); err != nil {
			logrus.Error(fmt.Sprintf("Unable to delete generated firewall rule %s: %s", ruleName, err.Error()))
			if strings.Contains(err.Error(), "No rules match the specified criteria") {
				time.Sleep(time.Millisecond * 200 * time.Duration(i+1))
				continue
			}
			return err
		}
		deleted = true
	}

	if !deleted {
		return err
	}

	return nil
}

func (e *EndpointManager) ApplyMappings(endpoint hcsshim.HNSEndpoint, mappings []netrules.PortMapping) (hcsshim.HNSEndpoint, error) {
	var policies []json.RawMessage
	if len(mappings) == 0 {
		return endpoint, nil
	}

	for _, mapping := range mappings {
		policy, err := json.Marshal(hcsshim.NatPolicy{
			Type:         hcsshim.Nat,
			Protocol:     "TCP",
			InternalPort: uint16(mapping.ContainerPort),
			ExternalPort: uint16(mapping.HostPort),
		})
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

	id := updatedEndpoint.Id
	var natAllocated bool
	var allocatedEndpoint *hcsshim.HNSEndpoint

	for i := 0; i < 10; i++ {
		natAllocated = false
		allocatedEndpoint, err = e.hcsClient.GetHNSEndpointByID(id)

		for _, a := range allocatedEndpoint.Resources.Allocators {
			if a.Type == hcsshim.NATPolicyType {
				natAllocated = true
				break
			}
		}

		if natAllocated {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	if !natAllocated {
		return hcsshim.HNSEndpoint{}, errors.New("NAT not initialized in time")
	}

	return *allocatedEndpoint, nil
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
