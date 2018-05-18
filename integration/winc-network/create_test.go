package main_test

import (
	"fmt"
	"net"
	"os"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/netinterface"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	var (
		n       netinterface.NetInterface
		localIp string
	)

	BeforeEach(func() {
		networkConfig = helpers.GenerateNetworkConfig()
		n = netinterface.NetInterface{}

		var err error
		localIp, err = localip.LocalIP()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		failed = failed || CurrentGinkgoTestDescription().Failed
		helpers.DeleteNetwork(networkConfig, networkConfigFile)
		Expect(os.Remove(networkConfigFile)).To(Succeed())
	})

	It("creates the network with the correct name", func() {
		helpers.CreateNetwork(networkConfig, networkConfigFile)

		natAdapter, err := n.ByName(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
		Expect(err).ToNot(HaveOccurred())
		Expect(natAdapter.Name).To(Equal(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName)))
	})

	It("creates the network with the correct subnet range", func() {
		helpers.CreateNetwork(networkConfig, networkConfigFile)

		natAdapter, err := n.ByName(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
		Expect(err).ToNot(HaveOccurred())

		addrs, err := natAdapter.Addrs()
		Expect(err).ToNot(HaveOccurred())

		ipV4nets := []*net.IPNet{}
		for _, a := range addrs {
			_, network, err := net.ParseCIDR(a.String())
			Expect(err).ToNot(HaveOccurred())
			if network.IP.To4() != nil {
				ipV4nets = append(ipV4nets, network)
			}
		}
		Expect(len(ipV4nets)).To(Equal(1))
		Expect(ipV4nets[0].String()).To(Equal(networkConfig.SubnetRange))
	})

	It("creates the network with the correct gateway address", func() {
		helpers.CreateNetwork(networkConfig, networkConfigFile)

		natAdapter, err := n.ByName(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
		Expect(err).ToNot(HaveOccurred())

		addrs, err := natAdapter.Addrs()
		Expect(err).ToNot(HaveOccurred())

		ipV4addrs := []net.IP{}
		for _, a := range addrs {
			ip, _, err := net.ParseCIDR(a.String())
			Expect(err).ToNot(HaveOccurred())
			if ipv4 := ip.To4(); ipv4 != nil {
				ipV4addrs = append(ipV4addrs, ipv4)
			}
		}
		Expect(len(ipV4addrs)).To(Equal(1))
		Expect(ipV4addrs[0].String()).To(Equal(networkConfig.GatewayAddress))
	})

	It("creates the network with mtu matching that of the host", func() {
		hostAdapter, err := n.ByIP(localIp)
		Expect(err).ToNot(HaveOccurred())

		helpers.CreateNetwork(networkConfig, networkConfigFile)

		virtualMtu, err := n.GetMTU(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
		Expect(err).ToNot(HaveOccurred())

		Expect(virtualMtu).To(Equal(uint32(hostAdapter.MTU)))
	})

	Context("mtu is set in the config", func() {
		BeforeEach(func() {
			networkConfig.MTU = 1400
		})

		It("creates the network with the configured mtu", func() {
			helpers.CreateNetwork(networkConfig, networkConfigFile)

			virtualMtu, err := n.GetMTU(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
			Expect(err).ToNot(HaveOccurred())

			Expect(virtualMtu).To(Equal(uint32(networkConfig.MTU)))
		})
	})
})
