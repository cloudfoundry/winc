package main_test

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Down", func() {
	BeforeEach(func() {
		fmt.Println(containerId)
		bundleSpec := helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))

		helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		networkConfig = helpers.GenerateNetworkConfig()
		helpers.CreateNetwork(networkConfig, networkConfigFile)

		helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {}}`, networkConfigFile)
		Expect(len(allEndpoints(containerId))).To(Equal(1))
	})

	AfterEach(func() {
		deleteContainerAndNetwork(containerId, networkConfig)
	})

	It("deletes the endpoint", func() {
		cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(len(allEndpoints(containerId))).To(Equal(0))
		Expect(endpointExists(containerId)).To(BeFalse())
	})

	It("deletes the associated firewall rules", func() {
		cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))

		getFirewallRule := fmt.Sprintf(`Get-NetFirewallRule -DisplayName "%s"`, containerId)
		output, err = exec.Command("powershell.exe", "-Command", getFirewallRule).CombinedOutput()
		Expect(err).To(HaveOccurred())
		expectedOutput := fmt.Sprintf(`Get-NetFirewallRule : No MSFT_NetFirewallRule objects found with property 'DisplayName' equal to '%s'`, containerId)
		Expect(strings.Replace(string(output), "\r\n", "", -1)).To(ContainSubstring(expectedOutput))
	})

	Context("when the endpoint does not exist", func() {
		It("does nothing", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", "some-nonexistant-id")
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
		})
	})

	Context("when the container is deleted before the endpoint", func() {
		BeforeEach(func() {
			output, err := exec.Command(wincBin, "delete", containerId).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
		})

		It("deletes the endpoint", func() {
			Expect(endpointExists(containerId)).To(BeTrue())
			cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			Expect(endpointExists(containerId)).To(BeFalse())
		})
	})
})
