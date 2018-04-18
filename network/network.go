package network

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/netinterface"
	"code.cloudfoundry.org/winc/network/netrules"

	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter -o fakes/net_rule_applier.go --fake-name NetRuleApplier . NetRuleApplier
type NetRuleApplier interface {
	In(netrules.NetIn, string) (hcsshim.NatPolicy, hcsshim.ACLPolicy, error)
	Out(netrules.NetOut, string) (hcsshim.ACLPolicy, error)
	NatMTU(int) error
	ContainerMTU(int) error
	Cleanup() error
}

//go:generate counterfeiter -o fakes/endpoint_manager.go --fake-name EndpointManager . EndpointManager
type EndpointManager interface {
	Create() (hcsshim.HNSEndpoint, error)
	Delete() error
	ApplyMappings(hcsshim.HNSEndpoint, []hcsshim.NatPolicy, []hcsshim.ACLPolicy) (hcsshim.HNSEndpoint, error)
}

//go:generate counterfeiter -o fakes/hcs_client.go --fake-name HCSClient . HCSClient
type HCSClient interface {
	GetHNSNetworkByName(string) (*hcsshim.HNSNetwork, error)
	CreateNetwork(*hcsshim.HNSNetwork, func() (bool, error)) (*hcsshim.HNSNetwork, error)
	DeleteNetwork(*hcsshim.HNSNetwork) (*hcsshim.HNSNetwork, error)
}

type Config struct {
	MTU                      int      `json:"mtu"`
	NetworkName              string   `json:"network_name"`
	SubnetRange              string   `json:"subnet_range"`
	GatewayAddress           string   `json:"gateway_address"`
	DNSServers               []string `json:"dns_servers"`
	MaximumOutgoingBandwidth uint64   `json:"maximum_outgoing_bandwidth"`
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
	containerId     string
	config          Config
}

func NewNetworkManager(client HCSClient, applier NetRuleApplier, endpointManager EndpointManager, containerId string, config Config) *NetworkManager {
	return &NetworkManager{
		hcsClient:       client,
		applier:         applier,
		endpointManager: endpointManager,
		containerId:     containerId,
		config:          config,
	}
}

func (n *NetworkManager) CreateHostNATNetwork() error {
	existingNetwork, err := n.hcsClient.GetHNSNetworkByName(n.config.NetworkName)
	if err != nil {
		if _, isNotExist := err.(hcsshim.NetworkNotFoundError); !isNotExist {
			return err
		}
	}

	subnet := hcsshim.Subnet{AddressPrefix: n.config.SubnetRange, GatewayAddress: n.config.GatewayAddress}
	vsidPolicy, err := json.Marshal(hcsshim.VsidPolicy{
		Type: "VSID",
		VSID: 4096,
	})
	subnet.Policies = append(subnet.Policies, vsidPolicy)

	if err != nil {
		return err
	}

	subnets := []hcsshim.Subnet{subnet}

	if existingNetwork != nil {
		if len(existingNetwork.Subnets) == 1 && subnetsMatch(existingNetwork.Subnets[0], subnets[0]) {
			return nil
		}

		return &SameNATNetworkNameError{Name: n.config.NetworkName, Subnets: existingNetwork.Subnets}
	}

	network := &hcsshim.HNSNetwork{
		Name:         n.config.NetworkName,
		Type:         "overlay",
		Subnets:      subnets,
		AutomaticDNS: true,
	}

	localIP, err := localip.LocalIP()
	if err != nil {
		return err
	}

	ni := netinterface.NetInterface{}
	physicalInterface, err := ni.ByIP(localIP)
	if err != nil {
		return err
	}

	physicalName := physicalInterface.Name
	interfaceAlias := fmt.Sprintf("vEthernet (%s)", physicalName)

	networkReady := func() (bool, error) {
		exists, err := netinterface.InterfaceExists(interfaceAlias)
		if err != nil {
			return false, err
		}

		if !exists {
			return false, nil
		}

		if _, err := net.LookupHost("www.google.com"); err != nil {
			return false, nil
		}

		return true, nil
	}

	_, err = n.hcsClient.CreateNetwork(network, networkReady)
	return err

	// since we're just modifying host ethernet, probably don't need to change MTU
	//if err != nil {
	//	return err
	//}

	//return n.applier.NatMTU(n.config.MTU)
}

func subnetsMatch(a, b hcsshim.Subnet) bool {
	return (a.AddressPrefix == b.AddressPrefix) && (a.GatewayAddress == b.GatewayAddress)
}

func (n *NetworkManager) DeleteHostNATNetwork() error {
	network, err := n.hcsClient.GetHNSNetworkByName(n.config.NetworkName)
	if err != nil {
		if _, ok := err.(hcsshim.NetworkNotFoundError); ok {
			return nil
		}

		return err
	}
	_, err = n.hcsClient.DeleteNetwork(network)
	time.Sleep(10 * time.Second)
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

	createdEndpoint, err := n.endpointManager.Create()
	if err != nil {
		return outputs, err
	}
	ip := createdEndpoint.IPAddress.String()

	natPolicies := []hcsshim.NatPolicy{}
	mappedPorts := []netrules.PortMapping{}
	aclPolicies := []hcsshim.ACLPolicy{}

	for _, rule := range inputs.NetIn {
		nat, acl, err := n.applier.In(rule, ip)
		if err != nil {
			return outputs, err
		}

		natPolicies = append(natPolicies, nat)
		aclPolicies = append(aclPolicies, acl)
		mappedPorts = append(mappedPorts, netrules.PortMapping{HostPort: uint32(nat.ExternalPort), ContainerPort: uint32(nat.InternalPort)})
	}

	for _, dnsServer := range n.config.DNSServers {
		serverIP := net.ParseIP(dnsServer)
		inputs.NetOut = append(inputs.NetOut,
			netrules.NetOut{
				Protocol: netrules.ProtocolTCP,
				Networks: []netrules.IPRange{{Start: serverIP, End: serverIP}},
				Ports:    []netrules.PortRange{{Start: 53, End: 53}},
			},
			netrules.NetOut{
				Protocol: netrules.ProtocolUDP,
				Networks: []netrules.IPRange{{Start: serverIP, End: serverIP}},
				Ports:    []netrules.PortRange{{Start: 53, End: 53}},
			},
		)
	}

	for _, rule := range inputs.NetOut {
		p, err := n.applier.Out(rule, ip)
		if err != nil {
			return outputs, err
		}
		aclPolicies = append(aclPolicies, p)
	}

	if _, err := n.endpointManager.ApplyMappings(createdEndpoint, natPolicies, aclPolicies); err != nil {
		return outputs, err
	}

	//	if err := n.applier.ContainerMTU(n.config.MTU); err != nil {
	//		return outputs, err
	//	}

	portBytes, err := json.Marshal(mappedPorts)
	if err != nil {
		return outputs, err
	}

	outputs.Properties.MappedPorts = string(portBytes)
	outputs.Properties.ContainerIP = ip
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
