//+build !acl

package endpoint

import (
	"fmt"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

func processAcls(acls []hcsshim.ACLPolicy) []hcsshim.ACLPolicy {
	return nil
}

func (e *EndpointManager) processEndpointPostAttach(endpoint *hcsshim.HNSEndpoint) error {
	var compartmentId uint32
	var endpointPortGuid string

	for _, a := range endpoint.Resources.Allocators {
		if a.Type == hcsshim.EndpointPortType {
			compartmentId = a.CompartmentId
			endpointPortGuid = a.EndpointPortGuid
			break
		}
	}

	if compartmentId == 0 || endpointPortGuid == "" {
		return fmt.Errorf("invalid endpoint %s allocators: %+v", endpoint.Id, endpoint.Resources.Allocators)
	}

	ruleName := fmt.Sprintf("Compartment %d - %s", compartmentId, endpointPortGuid)
	return e.deleteFirewallRule(ruleName)
}

func (e *EndpointManager) deleteFirewallRule(ruleName string) error {
	ruleCreated := false

	for i := 0; i < 3; i++ {
		var err error
		time.Sleep(time.Millisecond * 200 * time.Duration(i))

		ruleCreated, err = e.firewall.RuleExists(ruleName)
		if err != nil {
			return err
		}

		if ruleCreated {
			if err := e.firewall.DeleteRule(ruleName); err != nil {
				logrus.Error(fmt.Sprintf("Unable to delete generated firewall rule %s: %s", ruleName, err.Error()))
				return err
			}

			break
		}
	}

	if !ruleCreated {
		return fmt.Errorf("firewall rule %s not generated in time", ruleName)
	}

	return nil
}
