//+build acl

package endpoint

import (
	"code.cloudfoundry.org/winc/network/firewall"
	"github.com/Microsoft/hcsshim"
)

func processAcls(acls []hcsshim.ACLPolicy) []hcsshim.ACLPolicy {
	if len(acls) == 0 {
		// make sure everything's blocked if no netout rules present
		acls = []hcsshim.ACLPolicy{
			{
				Type:      hcsshim.ACL,
				Action:    hcsshim.Block,
				Direction: hcsshim.Out,
				Protocol:  uint16(firewall.NET_FW_IP_PROTOCOL_ANY),
			},
			{
				Type:      hcsshim.ACL,
				Action:    hcsshim.Block,
				Direction: hcsshim.In,
				Protocol:  uint16(firewall.NET_FW_IP_PROTOCOL_ANY),
			},
		}
	}

	return acls
}

func (e *EndpointManager) processEndpointPostAttach(endpoint *hcsshim.HNSEndpoint) error {
	return nil
}
