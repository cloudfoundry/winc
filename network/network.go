package network

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/netrules"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

const WINC_NETWORK = "winc-nat"
const SUBNET_RANGE = "172.30.0.0/22"
const GATEWAY_ADDRESS = "172.30.0.1"

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
	GetHNSEndpointByName(string) (*hcsshim.HNSEndpoint, error)
	DeleteEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
}

type Config struct {
	MTU int `json:"mtu"`
}

type UpInputs struct {
	Pid        int
	Properties map[string]interface{}
	NetOut     []netrules.NetOut `json:"netout_rules"`
	NetIn      []netrules.NetIn  `json:"netin"`
}

type UpOutputs struct {
	Properties struct {
		ContainerIP      string `json:"garden.network.container-ip"`
		DeprecatedHostIP string `json:"garden.network.host-ip"`
		MappedPorts      string `json:"garden.network.mapped-ports"`
	} `json:"properties"`
	DNSServers []string `json:"dns_servers,omitempty"`
}

//go:generate counterfeiter . NetRuleApplier
type NetRuleApplier interface {
	In(netrules.NetIn, *hcsshim.HNSEndpoint) (netrules.PortMapping, error)
	Out(netrules.NetOut, *hcsshim.HNSEndpoint) error
	MTU(string, int) error
	Cleanup() error
}

type Manager struct {
	hcsClient     HCSClient
	portAllocator PortAllocator
	applier       NetRuleApplier
	config        Config
	id            string
}

func NewManager(client HCSClient, portAllocator PortAllocator, applier NetRuleApplier, config Config, containerID string) *Manager {
	return &Manager{
		hcsClient:     client,
		portAllocator: portAllocator,
		applier:       applier,
		config:        config,
		id:            containerID,
	}
}

func (n *Manager) AttachEndpointToConfig(containerConfig hcsshim.ContainerConfig) (hcsshim.ContainerConfig, error) {
	wincNATNetwork, err := n.getWincNATNetwork()
	if err != nil {
		logrus.Error(err.Error())
		return hcsshim.ContainerConfig{}, err
	}

	appPortMapping, err := n.portMapping(8080)
	if err != nil {
		logrus.Error(err.Error())
		n.cleanupPorts()
		return hcsshim.ContainerConfig{}, err
	}

	sshPortMapping, err := n.portMapping(2222)
	if err != nil {
		logrus.Error(err.Error())
		n.cleanupPorts()
		return hcsshim.ContainerConfig{}, err
	}

	endpoint := &hcsshim.HNSEndpoint{
		Name:           n.id,
		VirtualNetwork: wincNATNetwork.Id,
		Policies:       []json.RawMessage{appPortMapping, sshPortMapping},
	}

	endpointID, err := n.createEndpoint(endpoint)
	if err != nil {
		logrus.Error(err.Error())
		n.cleanupPorts()
		return hcsshim.ContainerConfig{}, err
	}

	containerConfig.EndpointList = []string{endpointID}
	return containerConfig, nil
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

func (n *Manager) portMapping(containerPort int) (json.RawMessage, error) {
	hostPort, err := n.portAllocator.AllocatePort(n.id, 0)
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

func (n *Manager) DeleteContainerEndpoints(container hcs.Container) error {
	stats, err := container.Statistics()
	if err != nil {
		return err
	}

	var endpointIDs []string
	for _, network := range stats.Network {
		endpointIDs = append(endpointIDs, network.EndpointId)
	}

	return n.DeleteEndpointsById(endpointIDs)
}

func (n *Manager) DeleteEndpointsById(ids []string) error {
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

	return n.portAllocator.ReleaseAllPorts(n.id)
}

func (n *Manager) cleanupPorts() {
	releaseErr := n.portAllocator.ReleaseAllPorts(n.id)
	if releaseErr != nil {
		logrus.Error(releaseErr.Error())
	}
}

func (n *Manager) Up(inputs UpInputs) (UpOutputs, error) {
	outputs := UpOutputs{}

	if len(inputs.NetIn) > 2 {
		return outputs, fmt.Errorf("invalid number of port mappings: %d", len(inputs.NetIn))
	}

	endpoint, err := n.hcsClient.GetHNSEndpointByName(n.id)
	if err != nil {
		return outputs, err
	}

	mappedPorts := []netrules.PortMapping{}

	for _, rule := range inputs.NetIn {
		mapping, err := n.applier.In(rule, endpoint)
		if err != nil {
			return outputs, err
		}
		mappedPorts = append(mappedPorts, mapping)
	}

	for _, rule := range inputs.NetOut {
		if err := n.applier.Out(rule, endpoint); err != nil {
			return outputs, err
		}
	}

	if err := n.applier.MTU(endpoint.Id, n.config.MTU); err != nil {
		return outputs, err
	}

	portBytes, err := json.Marshal(mappedPorts)
	if err != nil {
		return outputs, err
	}

	outputs.Properties.MappedPorts = string(portBytes)
	outputs.Properties.ContainerIP, err = localip.LocalIP()
	if err != nil {
		return outputs, err
	}
	outputs.Properties.DeprecatedHostIP = "255.255.255.255"

	return outputs, nil
}

func (n *Manager) Down() error {
	return n.applier.Cleanup()
}
