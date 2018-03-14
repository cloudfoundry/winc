package main

import (
	"fmt"
	"net"
	"os"

	"github.com/Microsoft/hcsshim"
)

func main() {
	nets, err := hcsshim.HNSListNetworkRequest("GET", "", "")

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	for _, n := range nets {
		fmt.Printf("%+v\n", n)
		for _, p := range n.Subnets[0].Policies {
			fmt.Println(string(p))
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		fmt.Println(iface.Name)

		for _, addr := range addrs {
			fmt.Println(addr.String())
		}
	}
}
