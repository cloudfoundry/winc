// +build hns-acls

package main

import (
	"code.cloudfoundry.org/winc/hcs"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/endpoint"
	"code.cloudfoundry.org/winc/network/netrules"
	"code.cloudfoundry.org/winc/network/netsh"
	"code.cloudfoundry.org/winc/network/port_allocator"
)

func wireApplier(runner *netsh.Runner, handle string, portAlloctor *port_allocator.PortAllocator) (network.NetRuleApplier, error) {
	return netrules.NewApplier(runner, handle, portAlloctor), nil
}

func wireEndpointManager(hcsClient *hcs.Client, handle string, config network.Config) (network.EndpointManager, error) {
	return endpoint.NewEndpointManager(hcsClient, handle, config), nil
}
