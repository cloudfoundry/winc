package netrules_test

import (
	"encoding/json"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/netrules"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NetInterface", func() {
	var netIfaceFinder *netrules.NetInterface

	BeforeEach(func() {
		netIfaceFinder = &netrules.NetInterface{}
	})

	Describe("ByIP", func() {
		It("returns the physical adapter", func() {
			hostIPStr, err := localip.LocalIP()
			Expect(err).To(Succeed())

			iface, err := netIfaceFinder.ByIP(hostIPStr)
			Expect(err).To(Succeed())
			output, err := exec.Command("powershell.exe", "-Command", "(Get-NetAdapter -Physical) | Select-Object MacAddress,Name | ConvertTo-Json").CombinedOutput()
			Expect(err).To(Succeed())
			psIface := struct {
				MacAddress string `json:"MacAddress"`
				Name       string `json:"Name"`
			}{}
			Expect(json.Unmarshal(output, &psIface)).To(Succeed())

			Expect(iface.HardwareAddr.String()).To(Equal(strings.ToLower(strings.Replace(psIface.MacAddress, "-", ":", -1))))
			Expect(iface.Name).To(Equal(psIface.Name))
		})
		Context("when no physical adapter was found", func() {
			It("returns a descriptive error", func() {
				_, err := netIfaceFinder.ByIP("1.1.1.1")
				Expect(err).To(MatchError("unable to find interface for IP: 1.1.1.1"))
			})
		})
	})
})
