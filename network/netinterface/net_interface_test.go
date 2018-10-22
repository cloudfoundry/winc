package netinterface_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/netinterface"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/windows"
)

var _ = Describe("NetInterface", func() {
	var (
		netIface    *netinterface.NetInterface
		adapterName string
	)

	BeforeEach(func() {
		ifaces, err := net.Interfaces()
		Expect(err).To(Succeed())
		adapterName = ifaces[0].Name

		netIface = &netinterface.NetInterface{}
	})

	Describe("ByName", func() {
		type psAdapterInfo struct {
			Name    string `json:"Name"`
			IfIndex uint32 `json:"IfIndex"`
			LUID    uint64 `json:"NetLuid"`
		}

		var (
			psAdapter psAdapterInfo
		)

		BeforeEach(func() {
			output, err := exec.Command("powershell.exe", "-Command", fmt.Sprintf("(Get-NetAdapter -Name %s) | Select-Object IfIndex,Name,NetLuid | ConvertTo-Json", adapterName)).CombinedOutput()
			Expect(err).To(Succeed())

			Expect(json.Unmarshal(output, &psAdapter)).To(Succeed())
		})

		It("returns the adapter with that name", func() {
			iface, err := netIface.ByName(adapterName)
			Expect(err).To(Succeed())

			Expect(iface.Name).To(Equal(psAdapter.Name))
			Expect(iface.Index).To(Equal(psAdapter.IfIndex))
			Expect(iface.LUID.Value).To(Equal(psAdapter.LUID))
		})

		Context("when an adapter by that name is not found", func() {
			It("returns a descriptive error", func() {
				_, err := netIface.ByName("no-such-adapter")
				Expect(err).To(BeAssignableToTypeOf(&netinterface.InterfaceNotFoundError{}))
			})
		})
	})

	Describe("ByIP", func() {
		It("returns the physical adapter when given the host IP", func() {
			hostIPStr, err := localip.LocalIP()
			Expect(err).To(Succeed())

			iface, err := netIface.ByIP(hostIPStr)
			Expect(err).To(Succeed())
			output, err := exec.Command("powershell.exe", "-Command", "(Get-NetAdapter -Physical) | Select-Object MacAddress,Name | ConvertTo-Json").CombinedOutput()
			Expect(err).To(Succeed())
			psIface := struct {
				MacAddress string `json:"MacAddress"`
				Name       string `json:"Name"`
			}{}
			Expect(json.Unmarshal(output, &psIface)).To(Succeed())

			Expect(iface.PhysicalAddress.String()).To(Equal(strings.ToLower(strings.Replace(psIface.MacAddress, "-", ":", -1))))
			Expect(iface.Name).To(Equal(psIface.Name))
		})
		Context("when no physical adapter was found", func() {
			It("returns a descriptive error", func() {
				_, err := netIface.ByIP("1.1.1.1")
				Expect(err).To(BeAssignableToTypeOf(&netinterface.InterfaceForIPNotFoundError{}))
			})
		})
	})

	Describe("GetMTU", func() {
		var (
			interfaceMTU uint32
		)

		BeforeEach(func() {
			output, err := exec.Command("powershell.exe", "-Command", fmt.Sprintf(`(Get-NetIPInterface -AddressFamily Ipv4 -InterfaceAlias "%s").NlMtu`, adapterName)).CombinedOutput()
			Expect(err).To(Succeed())

			mtu, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 32)
			Expect(err).To(Succeed())
			interfaceMTU = uint32(mtu)
		})

		Context("IPv4", func() {
			It("returns the correct MTU for IPV4 interface", func() {
				mtu, err := netIface.GetMTU(adapterName, windows.AF_INET)
				Expect(err).To(Succeed())
				Expect(mtu).To(Equal(interfaceMTU))
			})
		})
	})
})
