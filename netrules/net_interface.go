package netrules

import (
	"fmt"
	"net"
)

type NetInterface struct{}

func (n *NetInterface) ByName(name string) (*net.Interface, error) {
	return net.InterfaceByName(name)
}

func (n *NetInterface) ByIP(ipStr string) (*net.Interface, error) {
	ip := net.ParseIP(ipStr)

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			_, net, err := net.ParseCIDR(addr.String())
			if err != nil {
				return nil, err
			}

			if net.Contains(ip) {
				return &iface, nil
			}
		}
	}
	return nil, fmt.Errorf("unable to find interface for IP: %s", ipStr)
}
