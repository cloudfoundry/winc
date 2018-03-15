package netrules

import (
	"fmt"
	"net"
	"strconv"
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

func (a *Applier) In(rule NetIn, containerIP string) (PortMapping, error) {
	externalPort := rule.HostPort
	mapping := PortMapping{}

	if externalPort == 0 {
		allocatedPort, err := a.portAllocator.AllocatePort(a.containerId, 0)
		if err != nil {
			return mapping, err
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
		return mapping, err
	}

	if err := a.openPort(rule.ContainerPort); err != nil {
		return mapping, err
	}

	return PortMapping{
		ContainerPort: rule.ContainerPort,
		HostPort:      externalPort,
	}, nil
}

func (a *Applier) Out(rule NetOut, containerIP string) (hcsshim.ACLPolicy, error) {
	p := hcsshim.ACLPolicy{
		Type:      hcsshim.ACL,
		Action:    hcsshim.Allow,
		Direction: hcsshim.Out,
		//	LocalAddresses: containerIP + "/32",
		RuleType: hcsshim.Switch,
	}

	remoteAddresses := []string{}
	for _, r := range rule.Networks {
		remoteAddresses = append(remoteAddresses, strings.Join(IPRangeToCIDRs(r), ", "))
	}

	p.RemoteAddresses = strings.Join(remoteAddresses, ", ")

	switch rule.Protocol {
	case ProtocolTCP:
		p.RemotePort = firewallRulePortRange(rule.Ports)
		p.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_TCP)
	case ProtocolUDP:
		p.RemotePort = firewallRulePortRange(rule.Ports)
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
