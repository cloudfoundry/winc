package network

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/netrules"

	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter . NetRuleApplier
type NetRuleApplier interface {
	In(netrules.NetIn, *hcsshim.HNSEndpoint) (netrules.PortMapping, error)
	Out(netrules.NetOut, *hcsshim.HNSEndpoint) error
	MTU(string, int) error
	Cleanup() error
}

type Config struct {
	MTU            int      `json:"mtu"`
	NetworkName    string   `json:"network_name"`
	SubnetRange    string   `json:"subnet_range"`
	GatewayAddress string   `json:"gateway_address"`
	DNSServers     []string `json:"dns_servers"`
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

type NetworkManager struct {
	hcsClient   HCSClient
	applier     NetRuleApplier
	containerId string
	config      Config
}

func NewNetworkManager(client HCSClient, applier NetRuleApplier, containerId string, config Config) *NetworkManager {
	return &NetworkManager{
		hcsClient:   client,
		applier:     applier,
		containerId: containerId,
		config:      config,
	}
}

func (n *NetworkManager) CreateHostNATNetwork() error {
	existingNetwork, err := n.hcsClient.GetHNSNetworkByName(n.config.NetworkName)
	if err != nil && !strings.Contains(err.Error(), "Network "+n.config.NetworkName+" not found") {
		return err
	}

	subnets := []hcsshim.Subnet{{AddressPrefix: n.config.SubnetRange, GatewayAddress: n.config.GatewayAddress}}

	if existingNetwork != nil {
		if len(existingNetwork.Subnets) == 1 && subnetsMatch(existingNetwork.Subnets[0], subnets[0]) {
			return nil
		}

		return &SameNATNetworkNameError{Name: n.config.NetworkName, Subnets: existingNetwork.Subnets}
	}

	network := &hcsshim.HNSNetwork{
		Name:    n.config.NetworkName,
		Type:    "nat",
		Subnets: subnets,
	}

	if len(n.config.DNSServers) > 0 {
		network.DNSServerList = strings.Join(n.config.DNSServers, ",")
	}

	_, err = n.hcsClient.CreateNetwork(network)
	return err
}

func subnetsMatch(a, b hcsshim.Subnet) bool {
	return (a.AddressPrefix == b.AddressPrefix) && (a.GatewayAddress == b.GatewayAddress)
}

func (n *NetworkManager) DeleteHostNATNetwork() error {
	network, err := n.hcsClient.GetHNSNetworkByName(n.config.NetworkName)
	if err != nil {
		if err.Error() == fmt.Sprintf("Network %s not found", n.config.NetworkName) {
			return nil
		}

		return err
	}
	_, err = n.hcsClient.DeleteNetwork(network)
	return err
}

func (n *NetworkManager) Up(inputs UpInputs) (UpOutputs, error) {
	outputs := UpOutputs{}

	if len(inputs.NetIn) > 2 {
		return outputs, fmt.Errorf("invalid number of port mappings: %d", len(inputs.NetIn))
	}

	endpoint, err := n.hcsClient.GetHNSEndpointByName(n.containerId)
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

	interfaceId := n.containerId
	if err := n.applier.MTU(interfaceId, n.config.MTU); err != nil {
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

func (n *NetworkManager) Down() error {
	return n.applier.Cleanup()
}
