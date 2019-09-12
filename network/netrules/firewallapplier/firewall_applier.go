package firewallapplier

import (
	"fmt"
	"strconv"

	"code.cloudfoundry.org/winc/network/firewall"
	"code.cloudfoundry.org/winc/network/netrules"
	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter -o fakes/netsh_runner.go --fake-name NetShRunner . NetShRunner
type NetShRunner interface {
	RunContainer([]string) error
}

//go:generate counterfeiter -o fakes/port_allocator.go --fake-name PortAllocator . PortAllocator
type PortAllocator interface {
	AllocatePort(handle string, port int) (int, error)
	ReleaseAllPorts(handle string) error
}

//go:generate counterfeiter -o fakes/firewall.go --fake-name Firewall . Firewall
type Firewall interface {
	CreateRule(firewall.Rule) error
	DeleteRule(string) error
	RuleExists(string) (bool, error)
}

type Applier struct {
	netSh         NetShRunner
	containerId   string
	portAllocator PortAllocator
	firewall      Firewall
}

func NewApplier(netSh NetShRunner, containerId string, portAllocator PortAllocator, firewall Firewall) *Applier {
	return &Applier{
		netSh:         netSh,
		containerId:   containerId,
		portAllocator: portAllocator,
		firewall:      firewall,
	}
}

func (a *Applier) In(rule netrules.NetIn, containerIP string) (*hcsshim.NatPolicy, *hcsshim.ACLPolicy, error) {
	externalPort := rule.HostPort

	if externalPort == 0 {
		allocatedPort, err := a.portAllocator.AllocatePort(a.containerId, 0)
		if err != nil {
			return nil, nil, err
		}
		externalPort = uint32(allocatedPort)
	}

	fr := firewall.Rule{
		Name:           a.containerId,
		Action:         firewall.NET_FW_ACTION_ALLOW,
		Direction:      firewall.NET_FW_RULE_DIR_IN,
		Protocol:       firewall.NET_FW_IP_PROTOCOL_TCP,
		LocalAddresses: containerIP,
		LocalPorts:     strconv.FormatUint(uint64(rule.ContainerPort), 10),
	}

	if err := a.firewall.CreateRule(fr); err != nil {
		return nil, nil, err
	}

	if err := a.OpenPort(rule.ContainerPort); err != nil {
		return nil, nil, err
	}

	return &hcsshim.NatPolicy{
		Type:         hcsshim.Nat,
		Protocol:     "TCP",
		InternalPort: uint16(rule.ContainerPort),
		ExternalPort: uint16(externalPort),
	}, nil, nil

}

func (a *Applier) Out(rule netrules.NetOut, containerIP string) (*hcsshim.ACLPolicy, error) {
	fr := firewall.Rule{
		Name:            a.containerId,
		Action:          firewall.NET_FW_ACTION_ALLOW,
		Direction:       firewall.NET_FW_RULE_DIR_OUT,
		LocalAddresses:  containerIP,
		RemoteAddresses: netrules.FirewallRuleIPRange(rule.Networks),
	}

	switch rule.Protocol {
	case netrules.ProtocolTCP:
		fr.RemotePorts = netrules.FirewallRulePortRange(rule.Ports)
		fr.Protocol = firewall.NET_FW_IP_PROTOCOL_TCP
	case netrules.ProtocolUDP:
		fr.RemotePorts = netrules.FirewallRulePortRange(rule.Ports)
		fr.Protocol = firewall.NET_FW_IP_PROTOCOL_UDP
	case netrules.ProtocolICMP:
		fr.Protocol = firewall.NET_FW_IP_PROTOCOL_ICMP
	case netrules.ProtocolAll:
		fr.Protocol = firewall.NET_FW_IP_PROTOCOL_ANY
	default:
		return nil, fmt.Errorf("invalid protocol: %d", rule.Protocol)
	}

	return nil, a.firewall.CreateRule(fr)
}

func (a *Applier) Cleanup() error {
	portReleaseErr := a.portAllocator.ReleaseAllPorts(a.containerId)

	// we can just delete the rule here since it will succeed
	// if the rule does not exist
	deleteErr := a.firewall.DeleteRule(a.containerId)

	if portReleaseErr != nil && deleteErr != nil {
		return fmt.Errorf("%s, %s", portReleaseErr.Error(), deleteErr.Error())
	}
	if portReleaseErr != nil {
		return portReleaseErr
	}
	if deleteErr != nil {
		return deleteErr
	}

	return nil
}

func (a *Applier) OpenPort(port uint32) error {
	args := []string{"http", "add", "urlacl", fmt.Sprintf("url=http://*:%d/", port), "user=Users"}
	return a.netSh.RunContainer(args)
}
