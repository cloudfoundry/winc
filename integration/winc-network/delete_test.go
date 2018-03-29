package main_test

import (
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {
	BeforeEach(func() {
		networkConfig = helpers.GenerateNetworkConfig()
		helpers.CreateNetwork(networkConfig, networkConfigFile)

	})

	It("deletes the NAT network", func() {
		helpers.DeleteNetwork(networkConfig, networkConfigFile)

		psCommand := fmt.Sprintf(`(Get-NetAdapter -name "vEthernet (%s)").InterfaceAlias`, networkConfig.NetworkName)
		output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		expectedOutput := fmt.Sprintf("Get-NetAdapter : No MSFT_NetAdapter objects found with property 'Name' equal to 'vEthernet (%s)'", networkConfig.NetworkName)
		Expect(strings.Replace(string(output), "\r\n", "", -1)).To(ContainSubstring(expectedOutput))
	})
})
