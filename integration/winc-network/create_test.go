package main_test

import (
	"fmt"
	"net"
	"os"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/netinterface"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/windows"
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

		natAdapter, err := net.InterfaceByName(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
		Expect(err).ToNot(HaveOccurred())
		Expect(natAdapter.Name).To(Equal(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName)))
	})

	It("creates the network with the correct subnet range", func() {
		helpers.CreateNetwork(networkConfig, networkConfigFile)

		natAdapter, err := net.InterfaceByName(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
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

		natAdapter, err := net.InterfaceByName(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
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

	It("creates the network with mtu matching that of the host IPv4 interface", func() {
		hostAdapter, err := n.ByIP(localIp)
		Expect(err).ToNot(HaveOccurred())

		hostMtu, err := n.GetMTU(hostAdapter.Name, windows.AF_INET)
		Expect(err).ToNot(HaveOccurred())

		helpers.CreateNetwork(networkConfig, networkConfigFile)

		natMtu, err := n.GetMTU(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName), windows.AF_INET)
		Expect(err).ToNot(HaveOccurred())

		Expect(natMtu).To(Equal(hostMtu))
	})

	Context("DNS Suffix Search List is set in the config", func() {
		BeforeEach(func() {
			networkConfig.DNSSuffix = []string{"example1.dns-suffix", "example2.dns-suffix"}
			/*
			* Unlike other tests here, the network needs to be attached to a running container
			* because the DNSSuffix is only visible from within the container.
			 */
			bundleSpec := helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
		})

		AfterEach(func() {
			helpers.DeleteContainer(containerId)
		})

		It("creates the network with the configured DNS Suffix Search List", func() {
			helpers.CreateNetwork(networkConfig, networkConfigFile)

			_, err := net.InterfaceByName(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName))
			Expect(err).ToNot(HaveOccurred())

			helpers.NetworkUp(containerId, `{"Pid": 123, "netin": []}`, networkConfigFile)

			dnsSuffixCmd := fmt.Sprintf("(get-dnsclient -InterfaceAlias 'vEthernet (%s)').ConnectionSpecificSuffixSearchList", containerId)
			stdout, _, err := helpers.ExecInContainer(containerId, []string{"powershell", "-command", dnsSuffixCmd}, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout.String()).To(ContainSubstring("example1.dns-suffix"))
			Expect(stdout.String()).To(ContainSubstring("example2.dns-suffix"))
		})
	})

	Context("mtu is set in the config", func() {
		BeforeEach(func() {
			networkConfig.MTU = 1400
		})

		It("creates the network with the configured mtu", func() {
			helpers.CreateNetwork(networkConfig, networkConfigFile)

			natMtu, err := n.GetMTU(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName), windows.AF_INET)
			Expect(err).ToNot(HaveOccurred())

			Expect(natMtu).To(Equal(uint32(networkConfig.MTU)))
		})
	})
})
