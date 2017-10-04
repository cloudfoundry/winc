package network

import (
	"encoding/json"
	"fmt"

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

type NetworkManager struct {
	hcsClient   HCSClient
	applier     NetRuleApplier
	config      Config
	containerId string
}

func NewNetworkManager(client HCSClient, applier NetRuleApplier, config Config, containerId string) *NetworkManager {
	return &NetworkManager{
		hcsClient:   client,
		applier:     applier,
		config:      config,
		containerId: containerId,
	}
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

func (n *NetworkManager) Down() error {
	return n.applier.Cleanup()
}
