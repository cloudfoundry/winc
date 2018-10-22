package mtu

import (
	"fmt"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/netinterface"
	"golang.org/x/sys/windows"
)

//go:generate counterfeiter -o fakes/netinterface.go --fake-name NetInterface . NetInterface
type NetInterface interface {
	ByName(string) (netinterface.AdapterInfo, error)
	ByIP(string) (netinterface.AdapterInfo, error)
	SetMTU(string, uint32, uint32) error
	GetMTU(string, uint32) (uint32, error)
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
		adapterInfo, err := m.netInterface.ByName(fmt.Sprintf("vEthernet (%s)", m.networkName))
		if err != nil {
			return err
		}
		retMtu, err := m.netInterface.GetMTU(adapterInfo.Name, windows.AF_INET)
		if err != nil {
			return err
		}
		mtu = int(retMtu)
	}

	interfaceAlias := fmt.Sprintf("vEthernet (%s)", m.containerId)
	return m.netInterface.SetMTU(interfaceAlias, uint32(mtu), windows.AF_INET)
}

func (m *Mtu) SetNat(mtu int) error {
	if mtu == 0 {
		hostIP, err := localip.LocalIP()
		if err != nil {
			return err
		}
		adapterInfo, err := m.netInterface.ByIP(hostIP)
		if err != nil {
			return err
		}
		retMtu, err := m.netInterface.GetMTU(adapterInfo.Name, windows.AF_INET)
		if err != nil {
			return err
		}
		mtu = int(retMtu)
	}

	interfaceId := fmt.Sprintf("vEthernet (%s)", m.networkName)
	return m.netInterface.SetMTU(interfaceId, uint32(mtu), windows.AF_INET)
}
