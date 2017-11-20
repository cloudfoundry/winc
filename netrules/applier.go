package netrules

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/localip"
	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter . NetShRunner
type NetShRunner interface {
	RunContainer([]string) error
	RunHost([]string) ([]byte, error)
}

//go:generate counterfeiter . PortAllocator
type PortAllocator interface {
	AllocatePort(handle string, port int) (int, error)
	ReleaseAllPorts(handle string) error
}

//go:generate counterfeiter . NetIfaceFinder
type NetIfaceFinder interface {
	ByName(string) (*net.Interface, error)
	ByIP(string) (*net.Interface, error)
}

type Applier struct {
	netSh          NetShRunner
	containerId    string
	networkName    string
	portAllocator  PortAllocator
	netIfaceFinder NetIfaceFinder
}

func NewApplier(netSh NetShRunner, containerId string, networkName string, portAllocator PortAllocator, netIfaceFinder NetIfaceFinder) *Applier {
	return &Applier{
		netSh:          netSh,
		containerId:    containerId,
		networkName:    networkName,
		portAllocator:  portAllocator,
		netIfaceFinder: netIfaceFinder,
	}
}

func (a *Applier) In(rule NetIn) (hcsshim.NatPolicy, error) {
	externalPort := uint16(rule.HostPort)

	if externalPort == 0 {
		allocatedPort, err := a.portAllocator.AllocatePort(a.containerId, 0)
		if err != nil {
			return hcsshim.NatPolicy{}, err
		}
		externalPort = uint16(allocatedPort)
	}

	if err := a.openPort(rule.ContainerPort); err != nil {
		return hcsshim.NatPolicy{}, err
	}

	return hcsshim.NatPolicy{
		Type:         "NAT",
		Protocol:     "TCP",
		InternalPort: uint16(rule.ContainerPort),
		ExternalPort: externalPort,
	}, nil
}

func (a *Applier) Out(rule NetOut, endpoint hcsshim.HNSEndpoint) error {
	netShArgs := []string{
		"advfirewall", "firewall", "add", "rule",
		fmt.Sprintf(`name="%s"`, a.containerId),
		"dir=out",
		"action=allow",
		fmt.Sprintf("localip=%s", endpoint.IPAddress.String()),
		fmt.Sprintf("remoteip=%s", firewallRuleIPRange(rule.Networks)),
	}

	var protocol string
	switch rule.Protocol {
	case ProtocolTCP:
		protocol = "TCP"
		netShArgs = append(netShArgs, fmt.Sprintf("remoteport=%s", firewallRulePortRange(rule.Ports)))
	case ProtocolUDP:
		protocol = "UDP"
		netShArgs = append(netShArgs, fmt.Sprintf("remoteport=%s", firewallRulePortRange(rule.Ports)))
	case ProtocolICMP:
		protocol = "ICMP"
	case ProtocolAll:
		protocol = "ANY"
	default:
	}

	if protocol == "ICMP" {
		return nil
	}

	if protocol == "" {
		return fmt.Errorf("invalid protocol: %d", rule.Protocol)
	}

	netShArgs = append(netShArgs, fmt.Sprintf("protocol=%s", protocol))

	_, err := a.netSh.RunHost(netShArgs)
	return err
}

func (a *Applier) ContainerMTU(mtu int) error {
	if mtu == 0 {
		iface, err := a.netIfaceFinder.ByName(fmt.Sprintf("vEthernet (%s)", a.networkName))
		if err != nil {
			return err
		}
		mtu = iface.MTU
	}

	interfaceId := fmt.Sprintf(`"vEthernet (%s)"`, a.containerId)
	args := []string{"interface", "ipv4", "set", "subinterface", interfaceId, fmt.Sprintf("mtu=%d", mtu), "store=persistent"}

	return a.netSh.RunContainer(args)
}

func (a *Applier) NatMTU(mtu int) error {
	if mtu == 0 {
		hostIP, err := localip.LocalIP()
		if err != nil {
			return err
		}
		iface, err := a.netIfaceFinder.ByIP(hostIP)
		if err != nil {
			return err
		}
		mtu = iface.MTU
	}

	interfaceId := fmt.Sprintf(`vEthernet (%s)`, a.networkName)
	args := []string{"interface", "ipv4", "set", "subinterface", interfaceId, fmt.Sprintf("mtu=%d", mtu), "store=persistent"}

	_, err := a.netSh.RunHost(args)
	return err
}

func (a *Applier) openPort(port uint32) error {
	args := []string{"http", "add", "urlacl", fmt.Sprintf("url=http://*:%d/", port), "user=Users"}
	return a.netSh.RunContainer(args)
}

func (a *Applier) Cleanup() error {
	portReleaseErr := a.portAllocator.ReleaseAllPorts(a.containerId)

	existsArgs := []string{"advfirewall", "firewall", "show", "rule", fmt.Sprintf(`name="%s"`, a.containerId)}
	_, err := a.netSh.RunHost(existsArgs)
	if err != nil {
		return portReleaseErr
	}

	deleteArgs := []string{"advfirewall", "firewall", "delete", "rule", fmt.Sprintf(`name="%s"`, a.containerId)}
	_, deleteErr := a.netSh.RunHost(deleteArgs)

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
