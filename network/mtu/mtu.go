package mtu

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/localip"
)

//go:generate counterfeiter -o fakes/netinterface.go --fake-name NetInterface . NetInterface
type NetInterface interface {
	ByName(string) (*net.Interface, error)
	ByIP(string) (*net.Interface, error)
	SetMTU(string, int) error
}

type Mtu struct {
	containerId  string
	networkName  string
	netInterface NetInterface
}

func New(containerId string, networkName string, netInterface NetInterface) *Mtu {
	return &Mtu{
		containerId:  containerId,
		networkName:  networkName,
		netInterface: netInterface,
	}
}

func (m *Mtu) SetContainer(mtu int) error {
	if mtu == 0 {
		iface, err := m.netInterface.ByName(fmt.Sprintf("vEthernet (%s)", m.networkName))
		if err != nil {
			return err
		}
		mtu = iface.MTU
	}

	interfaceAlias := fmt.Sprintf("vEthernet (%s)", m.containerId)
	return m.netInterface.SetMTU(interfaceAlias, mtu)
}

func (m *Mtu) SetNat(mtu int) error {
	if mtu == 0 {
		hostIP, err := localip.LocalIP()
		if err != nil {
			return err
		}
		iface, err := m.netInterface.ByIP(hostIP)
		if err != nil {
			return err
		}
		mtu = iface.MTU
	}

	interfaceId := fmt.Sprintf("vEthernet (%s)", m.networkName)
	return m.netInterface.SetMTU(interfaceId, mtu)
}
