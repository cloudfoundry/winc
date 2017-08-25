package netrules

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter . NetShRunner
type NetShRunner interface {
	RunContainer([]string) error
	RunHost([]string) error
}

type Applier struct {
	netSh NetShRunner
	id    string
}

func NewApplier(netSh NetShRunner, containerId string) *Applier {
	return &Applier{
		netSh: netSh,
		id:    containerId,
	}
}

func (a *Applier) In(rule NetIn, endpoint *hcsshim.HNSEndpoint) (PortMapping, error) {
	portMapping := PortMapping{}

	if (rule.ContainerPort != 8080 && rule.ContainerPort != 2222) || rule.HostPort != 0 {
		return portMapping, fmt.Errorf("invalid port mapping: host %d, container %d", rule.HostPort, rule.ContainerPort)
	}

	for _, pol := range endpoint.Policies {
		natPolicy := hcsshim.NatPolicy{}
		if err := json.Unmarshal(pol, &natPolicy); err != nil {
			return portMapping, err
		}
		if natPolicy.Type == "NAT" && uint32(natPolicy.InternalPort) == rule.ContainerPort {
			portMapping = PortMapping{
				ContainerPort: uint32(natPolicy.InternalPort),
				HostPort:      uint32(natPolicy.ExternalPort),
			}

			break
		}
	}

	if err := a.openPort(portMapping.ContainerPort); err != nil {
		return portMapping, err
	}

	return portMapping, nil
}

func (a *Applier) Out(rule NetOut, endpoint *hcsshim.HNSEndpoint) error {
	netShArgs := []string{
		"advfirewall", "firewall", "add", "rule",
		fmt.Sprintf(`name="%s"`, a.id),
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

	return a.netSh.RunHost(netShArgs)
}

func (a *Applier) MTU(endpointId string, mtu int) error {
	if mtu == 0 {
		return nil
	}

	if mtu > 1500 {
		return fmt.Errorf("invalid mtu specified: %d", mtu)
	}

	interfaceName := fmt.Sprintf("vEthernet (Container NIC %s)", strings.Split(endpointId, "-")[0])
	args := []string{"interface", "ipv4", "set", "subinterface", fmt.Sprintf(`"%s"`, interfaceName), fmt.Sprintf("mtu=%d", mtu), "store=persistent"}

	return a.netSh.RunContainer(args)
}

func (a *Applier) openPort(port uint32) error {
	args := []string{"http", "add", "urlacl", fmt.Sprintf("url=http://*:%d/", port), "user=Users"}
	return a.netSh.RunContainer(args)
}

func (a *Applier) Cleanup() error {
	existsArgs := []string{"advfirewall", "firewall", "show", "rule", fmt.Sprintf(`name="%s"`, a.id)}
	if err := a.netSh.RunHost(existsArgs); err != nil {
		return nil
	}

	deleteArgs := []string{"advfirewall", "firewall", "delete", "rule", fmt.Sprintf(`name="%s"`, a.id)}

	return a.netSh.RunHost(deleteArgs)
}
