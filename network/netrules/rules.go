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

func IPRangeToCIDRs(iprange IPRange) []string {
	start := ipToUint(iprange.Start)
	end := ipToUint(iprange.End)
	r := []string{}

	for start <= end {
		maskLen := uint32(32)
		for maskLen > 0 {
			if start != first(start, maskLen-1) || end < last(start, maskLen-1) {
				break
			}
			maskLen--
		}

		r = append(r, cidrFromIntMask(start, maskLen))
		start = last(start, maskLen)
		if start == 0xffffffff {
			break
		}

		start++
	}

	return r
}

func ipToUint(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 + uint32(ip[1])<<16 + uint32(ip[2])<<8 + uint32(ip[3])
}

func cidrFromIntMask(start uint32, maskLen uint32) string {
	ip := net.IPv4(
		byte((start&0xff000000)>>24),
		byte((start&0x00ff0000)>>16),
		byte((start&0x0000ff00)>>8),
		byte(start&0x000000ff),
	)
	return fmt.Sprintf("%s/%d", ip.String(), maskLen)
}

func first(start uint32, maskLen uint32) uint32 {
	return start & bitMask(maskLen)
}

func last(start uint32, maskLen uint32) uint32 {
	return (start & bitMask(maskLen)) | (^bitMask(maskLen))
}

func bitMask(len uint32) uint32 {
	return 0xffffffff << (32 - len)
}
