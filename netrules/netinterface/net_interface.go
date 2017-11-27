package netinterface

import (
	"fmt"
	"net"
	"os/exec"
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

func (n *NetInterface) SetMTU(alias string, mtu int) error {
	output, err := exec.Command("powershell.exe", "-command", fmt.Sprintf(`$index = (Get-NetIPInterface -Includeallcompartments -interfaceAlias "%s").ifIndex; set-netipinterface -includeallcompartments -ifindex $index -nlmtubytes %d`, alias, mtu)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to set MTU. \n error: %s \n output: %s", err.Error(), string(output))
	}
	return nil
}
