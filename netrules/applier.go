package netrules

import (
	"fmt"
	"net"

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

func (a *Applier) In(rule NetIn) (*hcsshim.NatPolicy, *hcsshim.ACLPolicy, error) {
	externalPort := uint16(rule.HostPort)

	if externalPort == 0 {
		allocatedPort, err := a.portAllocator.AllocatePort(a.containerId, 0)
		if err != nil {
			return nil, nil, err
		}
		externalPort = uint16(allocatedPort)
	}

	if err := a.openPort(rule.ContainerPort); err != nil {
		return nil, nil, err
	}

	natPolicy := &hcsshim.NatPolicy{
		Type:         hcsshim.Nat,
		Protocol:     "TCP",
		InternalPort: uint16(rule.ContainerPort),
		ExternalPort: externalPort,
	}

	aclPolicy := &hcsshim.ACLPolicy{
		Type:      hcsshim.ACL,
		Protocol:  WindowsProtocolTCP,
		LocalPort: uint16(rule.ContainerPort),
		Action:    hcsshim.Allow,
		Direction: hcsshim.In,
	}

	return natPolicy, aclPolicy, nil
}

func (a *Applier) Out(rule NetOut) ([]*hcsshim.ACLPolicy, error) {
	aclPolicies := []*hcsshim.ACLPolicy{}
	ipCIDRs := []string{}

	for _, ipRange := range rule.Networks {
		ipCIDRs = append(ipCIDRs, IPRangeToCIDRs(ipRange)...)
	}

	if len(rule.Ports) == 0 {
		rule.Ports = []PortRange{{Start: 0, End: 0}}
	}

	if len(ipCIDRs) == 0 {
		ipCIDRs = []string{""}
	}

	for _, cidr := range ipCIDRs {
		if cidr == "0.0.0.0/0" {
			cidr = ""
		}

		for _, ports := range rule.Ports {
			for port := ports.Start; port <= ports.End; port++ {
				policy := &hcsshim.ACLPolicy{
					Type:            hcsshim.ACL,
					Direction:       hcsshim.Out,
					Action:          hcsshim.Allow,
					RemoteAddresses: cidr,
					RemotePort:      port,
				}
				policies := []*hcsshim.ACLPolicy{policy}

				switch rule.Protocol {
				case ProtocolTCP:
					policy.Protocol = WindowsProtocolTCP
				case ProtocolUDP:
					policy.Protocol = WindowsProtocolUDP
				case ProtocolICMP:
					policy.Protocol = WindowsProtocolICMP
					policy.RemotePort = 0
				case ProtocolAll:
					policy.Protocol = WindowsProtocolTCP
					policyUDP := &hcsshim.ACLPolicy{
						Type:            hcsshim.ACL,
						Direction:       hcsshim.Out,
						Action:          hcsshim.Allow,
						RemoteAddresses: cidr,
						RemotePort:      port,
						Protocol:        WindowsProtocolUDP,
					}

					policyICMP := &hcsshim.ACLPolicy{
						Type:            hcsshim.ACL,
						Direction:       hcsshim.Out,
						Action:          hcsshim.Allow,
						RemoteAddresses: cidr,
						Protocol:        WindowsProtocolICMP,
					}
					policies = append(policies, policyUDP, policyICMP)

				default:
					return nil, fmt.Errorf("invalid protocol: %d", rule.Protocol)
				}
				aclPolicies = append(aclPolicies, policies...)
			}
		}
	}

	return aclPolicies, nil
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
		iface, err := a.netIfaceFinder.ByName("Ethernet")
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
	return a.portAllocator.ReleaseAllPorts(a.containerId)
}
