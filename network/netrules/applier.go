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

func (a *Applier) In(rule NetIn, containerIP string) (hcsshim.NatPolicy, hcsshim.ACLPolicy, error) {
	externalPort := rule.HostPort

	if externalPort == 0 {
		allocatedPort, err := a.portAllocator.AllocatePort(a.containerId, 0)
		if err != nil {
			return hcsshim.NatPolicy{}, hcsshim.ACLPolicy{}, err
		}
		externalPort = uint32(allocatedPort)
	}

	if err := a.netInModifyHostVM(rule, containerIP); err != nil {
		return hcsshim.NatPolicy{}, hcsshim.ACLPolicy{}, err
	}

	if err := a.openPort(rule.ContainerPort); err != nil {
		return hcsshim.NatPolicy{}, hcsshim.ACLPolicy{}, err
	}

	return hcsshim.NatPolicy{
			Type:         hcsshim.Nat,
			Protocol:     "TCP",
			ExternalPort: uint16(externalPort),
			InternalPort: uint16(rule.ContainerPort),
		}, hcsshim.ACLPolicy{
			Type:           hcsshim.ACL,
			Action:         hcsshim.Allow,
			Direction:      hcsshim.In,
			Protocol:       uint16(firewall.NET_FW_IP_PROTOCOL_TCP),
			LocalAddresses: containerIP,
			LocalPorts:     strconv.FormatUint(uint64(rule.ContainerPort), 10),
		}, nil
}

func (a *Applier) Out(rule NetOut, containerIP string) (hcsshim.ACLPolicy, error) {
	lAddrs := []string{}

	for _, ipr := range rule.Networks {
		lAddrs = append(lAddrs, IPRangeToCIDRs(ipr)...)
	}

	// if any IP CIDRS are 0.0.0.0/0, all remote destinations are allowed.
	// However, passing 0.0.0.0/0 directly in our ACLPolicy doesn't actually
	// have that effect.
	// So just don't specfiy anything in our ACLPolicy -- this allows acces
	// to all remote destinations
	for _, addr := range lAddrs {
		if addr == "0.0.0.0/0" {
			lAddrs = []string{}
			break
		}
	}

	acl := hcsshim.ACLPolicy{
		Type:            hcsshim.ACL,
		Action:          hcsshim.Allow,
		Direction:       hcsshim.Out,
		LocalAddresses:  containerIP,
		RemoteAddresses: strings.Join(lAddrs, ","),
	}

	switch rule.Protocol {
	case ProtocolTCP:
		acl.RemotePorts = firewallRulePortRange(rule.Ports)
		acl.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_TCP)
	case ProtocolUDP:
		acl.RemotePorts = firewallRulePortRange(rule.Ports)
		acl.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_UDP)
	case ProtocolICMP:
		acl.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_ICMP)
	case ProtocolAll:
		acl.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_ANY)
	default:
		return hcsshim.ACLPolicy{}, fmt.Errorf("invalid protocol: %d", rule.Protocol)
	}

	if err := a.netOutModifyHostVM(rule, containerIP); err != nil {
		return hcsshim.ACLPolicy{}, err
	}

	return acl, nil
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

	cleanupErr := a.cleanupModifyHostVM()

	if portReleaseErr != nil && cleanupErr != nil {
		return fmt.Errorf("%s, %s", portReleaseErr.Error(), cleanupErr.Error())
	}
	if portReleaseErr != nil {
		return portReleaseErr
	}
	if cleanupErr != nil {
		return cleanupErr
	}

	return nil
}
