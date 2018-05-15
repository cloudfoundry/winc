//+build !acl

package netrules

import (
	"fmt"
	"strconv"

	"code.cloudfoundry.org/winc/network/firewall"
)

func (a *Applier) netInModifyHostVM(rule NetIn, containerIP string) error {
	fr := firewall.Rule{
		Name:           a.containerId,
		Action:         firewall.NET_FW_ACTION_ALLOW,
		Direction:      firewall.NET_FW_RULE_DIR_IN,
		Protocol:       firewall.NET_FW_IP_PROTOCOL_TCP,
		LocalAddresses: containerIP,
		LocalPorts:     strconv.FormatUint(uint64(rule.ContainerPort), 10),
	}

	return a.firewall.CreateRule(fr)
}

func (a *Applier) netOutModifyHostVM(rule NetOut, containerIP string) error {
	fr := firewall.Rule{
		Name:            a.containerId,
		Action:          firewall.NET_FW_ACTION_ALLOW,
		Direction:       firewall.NET_FW_RULE_DIR_OUT,
		LocalAddresses:  containerIP,
		RemoteAddresses: firewallRuleIPRange(rule.Networks),
	}

	switch rule.Protocol {
	case ProtocolTCP:
		fr.RemotePorts = firewallRulePortRange(rule.Ports)
		fr.Protocol = firewall.NET_FW_IP_PROTOCOL_TCP
	case ProtocolUDP:
		fr.RemotePorts = firewallRulePortRange(rule.Ports)
		fr.Protocol = firewall.NET_FW_IP_PROTOCOL_UDP
	case ProtocolICMP:
		fr.Protocol = firewall.NET_FW_IP_PROTOCOL_ICMP
	case ProtocolAll:
		fr.Protocol = firewall.NET_FW_IP_PROTOCOL_ANY
	default:
		return fmt.Errorf("invalid protocol: %d", rule.Protocol)
	}

	return a.firewall.CreateRule(fr)
}

func (a *Applier) cleanupModifyHostVM() error {
	// we can just delete the rule here since it will succeed
	// if the rule does not exist
	return a.firewall.DeleteRule(a.containerId)
}
