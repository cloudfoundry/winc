package netrules

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/localip"
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

//go:generate counterfeiter . NetInterface
type NetInterface interface {
	ByName(string) (*net.Interface, error)
	ByIP(string) (*net.Interface, error)
	SetMTU(string, int) error
}

type Applier struct {
	netSh         NetShRunner
	containerId   string
	networkName   string
	portAllocator PortAllocator
	netInterface  NetInterface
}

func NewApplier(netSh NetShRunner, containerId string, networkName string, portAllocator PortAllocator, netInterface NetInterface) *Applier {
	return &Applier{
		netSh:         netSh,
		containerId:   containerId,
		networkName:   networkName,
		portAllocator: portAllocator,
		netInterface:  netInterface,
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

	netShArgs := []string{
		"advfirewall", "firewall", "add", "rule",
		fmt.Sprintf(`name="%s"`, a.containerId),
		"dir=in",
		"action=allow",
		fmt.Sprintf("localip=%s", containerIP),
		fmt.Sprintf("localport=%d", rule.ContainerPort),
		"protocol=TCP",
	}

	_, err := a.netSh.RunHost(netShArgs)
	if err != nil {
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

func (a *Applier) Out(rule NetOut, containerIP string) error {
	netShArgs := []string{
		"advfirewall", "firewall", "add", "rule",
		fmt.Sprintf(`name="%s"`, a.containerId),
		"dir=out",
		"action=allow",
		fmt.Sprintf("localip=%s", containerIP),
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
		protocol = "ICMPV4"
	case ProtocolAll:
		protocol = "ANY"
	default:
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
