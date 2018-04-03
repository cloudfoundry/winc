package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	BeforeEach(func() {
		networkConfig = helpers.GenerateNetworkConfig()
	})

	AfterEach(func() {
		helpers.DeleteNetwork(networkConfig, networkConfigFile)
		Expect(os.Remove(networkConfigFile)).To(Succeed())
	})

	It("creates the network with the correct name", func() {
		helpers.CreateNetwork(networkConfig, networkConfigFile)

		psCommand := fmt.Sprintf(`(Get-NetAdapter -name "vEthernet (%s)").InterfaceAlias`, networkConfig.NetworkName)
		output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(strings.TrimSpace(string(output))).To(Equal(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName)))
	})

	It("creates the network with the correct subnet range", func() {
		helpers.CreateNetwork(networkConfig, networkConfigFile)

		psCommand := fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.IPAddress`, networkConfig.NetworkName)
		output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		ipAddress := strings.TrimSuffix(strings.TrimSpace(string(output)), "1") + "0"

		psCommand = fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.PrefixLength`, networkConfig.NetworkName)
		output, err = exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		prefixLength := strings.TrimSpace(string(output))

		Expect(fmt.Sprintf("%s/%s", ipAddress, prefixLength)).To(Equal(networkConfig.SubnetRange))
	})

	It("creates the network with the correct gateway address", func() {
		helpers.CreateNetwork(networkConfig, networkConfigFile)

		psCommand := fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.IPAddress`, networkConfig.NetworkName)
		output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(strings.TrimSpace(string(output))).To(Equal(networkConfig.GatewayAddress))
	})

	It("creates the network with mtu matching that of the host", func() {
		psCommand := `(Get-NetAdapter -Physical).Name`
		output, err := exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
		physicalNetworkName := strings.TrimSpace(string(output))

		psCommand = fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias '%s').NlMtu`, physicalNetworkName)
		output, err = exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
		physicalMTU := strings.TrimSpace(string(output))

		helpers.CreateNetwork(networkConfig, networkConfigFile)

		psCommand = fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, networkConfig.NetworkName)
		output, err = exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
		virtualMTU := strings.TrimSpace(string(output))

		Expect(virtualMTU).To(Equal(physicalMTU))
	})

	Context("mtu is set in the config", func() {
		BeforeEach(func() {
			networkConfig.MTU = 1400
		})

		It("creates the network with the configured mtu", func() {
			helpers.CreateNetwork(networkConfig, networkConfigFile)

			psCommand := fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, networkConfig.NetworkName)
			output, err := exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			virtualMTU := strings.TrimSpace(string(output))

			Expect(virtualMTU).To(Equal(strconv.Itoa(networkConfig.MTU)))
		})
	})
})
