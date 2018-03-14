package netrules

import (
	"fmt"
	"net"
	"strings"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/firewall"
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

//go:generate counterfeiter -o fakes/netinterface.go --fake-name NetInterface . NetInterface
type NetInterface interface {
	ByName(string) (*net.Interface, error)
	ByIP(string) (*net.Interface, error)
	SetMTU(string, int) error
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
	networkName   string
	portAllocator PortAllocator
	netInterface  NetInterface
	firewall      Firewall
}

func NewApplier(netSh NetShRunner, containerId string, networkName string, portAllocator PortAllocator, netInterface NetInterface, firewall Firewall) *Applier {
	return &Applier{
		netSh:         netSh,
		containerId:   containerId,
		networkName:   networkName,
		portAllocator: portAllocator,
		netInterface:  netInterface,
		firewall:      firewall,
	}
}

func (a *Applier) In(rule NetIn, containerIP string) (hcsshim.NatPolicy, hcsshim.ACLPolicy, error) {
	externalPort := uint16(rule.HostPort)

	if externalPort == 0 {
		allocatedPort, err := a.portAllocator.AllocatePort(a.containerId, 0)
		if err != nil {
			return hcsshim.NatPolicy{}, hcsshim.ACLPolicy{}, err
		}
		externalPort = uint16(allocatedPort)
	}

	if err := a.openPort(rule.ContainerPort); err != nil {
		return hcsshim.NatPolicy{}, hcsshim.ACLPolicy{}, err
	}

	return hcsshim.NatPolicy{
			Type:         hcsshim.Nat,
			Protocol:     "TCP",
			InternalPort: uint16(rule.ContainerPort),
			ExternalPort: externalPort,
		}, hcsshim.ACLPolicy{
			Type:           hcsshim.ACL,
			Action:         hcsshim.Allow,
			Direction:      hcsshim.In,
			LocalAddresses: containerIP,
			LocalPort:      uint16(rule.ContainerPort),
			RuleType:       hcsshim.Switch,
			Protocol:       uint16(firewall.NET_FW_IP_PROTOCOL_TCP),
		}, nil
}

func (a *Applier) Out(rule NetOut, containerIP string) (hcsshim.ACLPolicy, error) {
	p := hcsshim.ACLPolicy{
		Type:           hcsshim.ACL,
		Action:         hcsshim.Allow,
		Direction:      hcsshim.Out,
		LocalAddresses: containerIP,
		RuleType:       hcsshim.Switch,
	}

	remoteAddresses := []string{}
	for _, r := range rule.Networks {
		remoteAddresses = append(remoteAddresses, strings.Join(IPRangeToCIDRs(r), ", "))
	}

	if len(rule.Networks) > 0 {
		p.RemoteAddresses = rule.Networks[0].Start.String()
	}

	switch rule.Protocol {
	case ProtocolTCP:
		if len(rule.Ports) > 0 {
			p.RemotePort = rule.Ports[0].Start
		}

		//p.RemotePort = firewallRulePortRange(rule.Ports)
		p.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_TCP)
	case ProtocolUDP:
		if len(rule.Ports) > 0 {
			p.RemotePort = rule.Ports[0].Start
		}
		//p.RemotePort = firewallRulePortRange(rule.Ports)
		p.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_UDP)
	case ProtocolICMP:
		p.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_ICMP)
	case ProtocolAll:
		p.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_ANY)
	default:
		return hcsshim.ACLPolicy{}, fmt.Errorf("invalid protocol: %d", rule.Protocol)
	}

	return p, nil
}

func (a *Applier) ContainerMTU(mtu int) error {
	if mtu == 0 {
		iface, err := a.netInterface.ByName(fmt.Sprintf("vEthernet (%s)", a.networkName))
		if err != nil {
			return err
		}
		mtu = iface.MTU
	}

	interfaceAlias := fmt.Sprintf("vEthernet (%s)", a.containerId)
	return a.netInterface.SetMTU(interfaceAlias, mtu)
}

func (a *Applier) NatMTU(mtu int) error {
	if mtu == 0 {
		hostIP, err := localip.LocalIP()
		if err != nil {
			return err
		}
		iface, err := a.netInterface.ByIP(hostIP)
		if err != nil {
			return err
		}
		mtu = iface.MTU
	}

	interfaceId := fmt.Sprintf("vEthernet (%s)", a.networkName)
	return a.netInterface.SetMTU(interfaceId, mtu)
}

func (a *Applier) openPort(port uint32) error {
	args := []string{"http", "add", "urlacl", fmt.Sprintf("url=http://*:%d/", port), "user=Users"}
	return a.netSh.RunContainer(args)
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
