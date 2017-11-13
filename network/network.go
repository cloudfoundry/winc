package network

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/netrules"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

//go:generate counterfeiter . NetRuleApplier
type NetRuleApplier interface {
	In(netrules.NetIn) (*hcsshim.NatPolicy, *hcsshim.ACLPolicy, error)
	Out(netrules.NetOut) ([]*hcsshim.ACLPolicy, error)
	NatMTU(int) error
	ContainerMTU(int) error
	Cleanup() error
}

//go:generate counterfeiter . EndpointManager
type EndpointManager interface {
	Create([]*hcsshim.NatPolicy, []*hcsshim.ACLPolicy) error
	Delete() error
}

//go:generate counterfeiter . NetShRunner
type NetShRunner interface {
	RunContainer([]string) error
	RunHost([]string) ([]byte, error)
}

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	GetHNSNetworkByName(string) (*hcsshim.HNSNetwork, error)
	CreateNetwork(*hcsshim.HNSNetwork) (*hcsshim.HNSNetwork, error)
	DeleteNetwork(*hcsshim.HNSNetwork) (*hcsshim.HNSNetwork, error)
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
	hcsClient       HCSClient
	applier         NetRuleApplier
	endpointManager EndpointManager
	netshRunner     NetShRunner
	containerId     string
	config          Config
}

func NewNetworkManager(client HCSClient, applier NetRuleApplier, endpointManager EndpointManager, netshRunner NetShRunner, containerId string, config Config) *NetworkManager {
	return &NetworkManager{
		hcsClient:       client,
		applier:         applier,
		endpointManager: endpointManager,
		netshRunner:     netshRunner,
		containerId:     containerId,
		config:          config,
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

	_, err = n.hcsClient.CreateNetwork(network)
	if err != nil {
		return err
	}

	if err := n.applier.NatMTU(n.config.MTU); err != nil {
		return err
	}

	args := []string{"advfirewall", "firewall", "add", "rule", fmt.Sprintf("name=%s", n.config.NetworkName), "dir=in", "action=allow", fmt.Sprintf("localip=%s", n.config.SubnetRange), fmt.Sprintf("remoteip=%s", n.config.GatewayAddress)}
	_, err = n.netshRunner.RunHost(args)
	if err != nil {
		return fmt.Errorf("couldn't add firewall: %s", err.Error())
	}
	return nil
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

	args := []string{"advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s", n.config.NetworkName)}
	_, err = n.netshRunner.RunHost(args)
	if err != nil {
		logrus.Error(err.Error())
	}

	_, err = n.hcsClient.DeleteNetwork(network)
	return err
}

func (n *NetworkManager) Up(inputs UpInputs) (UpOutputs, error) {
	outputs, err := n.up(inputs)
	if err != nil {
		n.applier.Cleanup()
		n.endpointManager.Delete()
	}
	return outputs, err
}

func (n *NetworkManager) up(inputs UpInputs) (UpOutputs, error) {
	outputs := UpOutputs{}
	natPolicies := []*hcsshim.NatPolicy{}
	aclPolicies := []*hcsshim.ACLPolicy{}
	mappedPorts := []netrules.PortMapping{}

	for _, rule := range inputs.NetIn {
		natPolicy, aclPolicy, err := n.applier.In(rule)
		if err != nil {
			return outputs, err
		}
		natPolicies = append(natPolicies, natPolicy)
		aclPolicies = append(aclPolicies, aclPolicy)
		mapping := netrules.PortMapping{
			ContainerPort: uint32(natPolicy.InternalPort),
			HostPort:      uint32(natPolicy.ExternalPort),
		}
		mappedPorts = append(mappedPorts, mapping)
	}

	for _, rule := range inputs.NetOut {
		acls, err := n.applier.Out(rule)
		if err != nil {
			return outputs, err
		}
		aclPolicies = append(aclPolicies, acls...)
	}

	if err := n.endpointManager.Create(natPolicies, aclPolicies); err != nil {
		return outputs, err
	}

	if err := n.applier.ContainerMTU(n.config.MTU); err != nil {
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
	deleteErr := n.endpointManager.Delete()
	cleanupErr := n.applier.Cleanup()

	if deleteErr != nil && cleanupErr != nil {
		return fmt.Errorf("%s, %s", deleteErr.Error(), cleanupErr.Error())
	}

	if deleteErr != nil {
		return deleteErr
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	return nil
}
