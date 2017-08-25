package netrules

import (
	"fmt"
	"net"
	"strings"
)

type PortMapping struct {
	HostPort      uint32
	ContainerPort uint32
}

type NetIn struct {
	HostPort      uint32 `json:"host_port"`
	ContainerPort uint32 `json:"container_port"`
}

type NetOut struct {
	// the protocol to be whitelisted
	Protocol Protocol `json:"protocol,omitempty"`

	// a list of ranges of IP addresses to whitelist; Start to End inclusive; default all
	Networks []IPRange `json:"networks,omitempty"`

	// a list of ranges of ports to whitelist; Start to End inclusive; ignored if Protocol is ICMP; default all
	Ports []PortRange `json:"ports,omitempty"`
}

type Protocol uint8

const (
	ProtocolAll Protocol = iota
	ProtocolTCP
	ProtocolUDP
	ProtocolICMP
)

type IPRange struct {
	Start net.IP `json:"start,omitempty"`
	End   net.IP `json:"end,omitempty"`
}

func (ir IPRange) String() string {
	return fmt.Sprintf("%s-%s", ir.Start.String(), ir.End.String())
}

type PortRange struct {
	Start uint16 `json:"start,omitempty"`
	End   uint16 `json:"end,omitempty"`
}

func (pr PortRange) String() string {
	return fmt.Sprintf("%d-%d", pr.Start, pr.End)
}

// FirewallRuleIPRange create a valid ip range for windows firewall
func firewallRuleIPRange(networks []IPRange) string {
	var output []string
	for _, v := range networks {
		output = append(output, v.String())
	}
	return strings.Join(output, ",")
}

// FirewallRulePortRange create a valid port range for windows firewall
func firewallRulePortRange(ports []PortRange) string {
	var output []string
	for _, v := range ports {
		output = append(output, v.String())
	}
	return strings.Join(output, ",")
}
