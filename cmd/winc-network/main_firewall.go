// +build !hns-acls

package main

import (
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/endpoint/firewallendpoint"
	"code.cloudfoundry.org/winc/network/firewall"
	"code.cloudfoundry.org/winc/network/netrules/firewallapplier"
	"code.cloudfoundry.org/winc/network/netsh"
	"code.cloudfoundry.org/winc/network/port_allocator"
)

func wireApplier(runner *netsh.Runner, handle string, portAlloctor *port_allocator.PortAllocator) (network.NetRuleApplier, error) {
	f, err := firewall.NewFirewall("")
	if err != nil {
		return nil, err
	}

	return firewallapplier.NewApplier(runner, handle, portAlloctor, f), nil
}

func wireEndpointManager(hcsClient *hcs.Client, handle string, config network.Config) (network.EndpointManager, error) {
	f, err := firewall.NewFirewall("")
	if err != nil {
		return nil, err
	}

	return firewallendpoint.NewEndpointManager(hcsClient, f, handle, config), nil
}
