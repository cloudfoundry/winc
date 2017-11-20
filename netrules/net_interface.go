package netrules

import "net"

type NetInterface struct{}

func (n *NetInterface) ByName(name string) (*net.Interface, error) {
	return net.InterfaceByName(name)
}
