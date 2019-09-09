package netrules

import (
	"fmt"
	"strconv"
	"strings"

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

type Applier struct {
	netSh         NetShRunner
	containerId   string
	portAllocator PortAllocator
}

func NewApplier(netSh NetShRunner, containerId string, portAllocator PortAllocator) *Applier {
	return &Applier{
		netSh:         netSh,
		containerId:   containerId,
		portAllocator: portAllocator,
	}
}

func (a *Applier) In(rule NetIn, containerIP string) (*hcsshim.NatPolicy, *hcsshim.ACLPolicy, error) {
	externalPort := rule.HostPort

	if externalPort == 0 {
		allocatedPort, err := a.portAllocator.AllocatePort(a.containerId, 0)
		if err != nil {
			return nil, nil, err
		}
		externalPort = uint32(allocatedPort)
	}

	return &hcsshim.NatPolicy{
			Type:         hcsshim.Nat,
			Protocol:     "TCP",
			ExternalPort: uint16(externalPort),
			InternalPort: uint16(rule.ContainerPort),
		}, &hcsshim.ACLPolicy{
			Type:           hcsshim.ACL,
			Action:         hcsshim.Allow,
			Direction:      hcsshim.In,
			Protocol:       uint16(firewall.NET_FW_IP_PROTOCOL_TCP),
			LocalAddresses: containerIP,
			LocalPorts:     strconv.FormatUint(uint64(rule.ContainerPort), 10),
		}, nil
}

func (a *Applier) Out(rule NetOut, containerIP string) (*hcsshim.ACLPolicy, error) {
	rAddrs := []string{}

	for _, ipr := range rule.Networks {
		rAddrs = append(rAddrs, IPRangeToCIDRs(ipr)...)
	}

	// if any IP CIDRS are 0.0.0.0/0, all remote destinations are allowed.
	// However, passing 0.0.0.0/0 directly in our ACLPolicy doesn't actually
	// have that effect.
	// So just don't specfiy anything in our ACLPolicy -- this allows acces
	// to all remote destinations
	for _, addr := range rAddrs {
		if addr == "0.0.0.0/0" {
			rAddrs = []string{}
			break
		}
	}

	acl := hcsshim.ACLPolicy{
		Type:            hcsshim.ACL,
		Action:          hcsshim.Allow,
		Direction:       hcsshim.Out,
		LocalAddresses:  containerIP,
		RemoteAddresses: strings.Join(rAddrs, ","),
	}

	switch rule.Protocol {
	case ProtocolTCP:
		acl.RemotePorts = FirewallRulePortRange(rule.Ports)
		acl.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_TCP)
	case ProtocolUDP:
		acl.RemotePorts = FirewallRulePortRange(rule.Ports)
		acl.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_UDP)
	case ProtocolICMP:
		acl.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_ICMP)
	case ProtocolAll:
		acl.Protocol = uint16(firewall.NET_FW_IP_PROTOCOL_ANY)
	default:
		return nil, fmt.Errorf("invalid protocol: %d", rule.Protocol)
	}

	return &acl, nil
}

func (a *Applier) OpenPort(port uint32) error {
	args := []string{"http", "add", "urlacl", fmt.Sprintf("url=http://*:%d/", port), "user=Users"}
	return a.netSh.RunContainer(args)
}
func (a *Applier) Cleanup() error {
	return a.portAllocator.ReleaseAllPorts(a.containerId)
}
