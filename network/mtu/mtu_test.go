package mtu_test

import (
	"fmt"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/mtu"
	"code.cloudfoundry.org/winc/network/mtu/fakes"
	"code.cloudfoundry.org/winc/network/netinterface"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/windows"
)

var _ = Describe("Mtu", func() {
	const containerId = "containerabc"
	const networkName = "my-network"

	var (
		netInterface *fakes.NetInterface
		m            *mtu.Mtu
	)

	BeforeEach(func() {
		netInterface = &fakes.NetInterface{}
		m = mtu.New(containerId, networkName, netInterface)
	})

	Describe("SetContainer", func() {
		It("applies the mtu to the container", func() {
			Expect(m.SetContainer(1405)).To(Succeed())

			Expect(netInterface.SetMTUCallCount()).To(Equal(1))
			alias, mtu, family := netInterface.SetMTUArgsForCall(0)
			Expect(alias).To(Equal("vEthernet (containerabc)"))
			Expect(mtu).To(Equal(uint32(1405)))
			Expect(family).To(Equal(uint32(windows.AF_INET)))
		})

		Context("the specified mtu is 0", func() {
			var natNetworkName string

			BeforeEach(func() {
				natNetworkName = fmt.Sprintf(`vEthernet (%s)`, networkName)
				netInterface.ByNameReturns(netinterface.AdapterInfo{Name: natNetworkName}, nil)
				netInterface.GetMTUReturns(1302, nil)
			})

			It("sets the container MTU to the NAT network MTU", func() {
				Expect(m.SetContainer(0)).To(Succeed())

				Expect(netInterface.ByNameCallCount()).To(Equal(1))
				Expect(netInterface.ByNameArgsForCall(0)).To(Equal(natNetworkName))

				Expect(netInterface.GetMTUCallCount()).To(Equal(1))
				alias, family := netInterface.GetMTUArgsForCall(0)
				Expect(alias).To(Equal(natNetworkName))
				Expect(family).To(Equal(uint32(windows.AF_INET)))

				Expect(netInterface.SetMTUCallCount()).To(Equal(1))
				alias, mtu, family := netInterface.SetMTUArgsForCall(0)
				Expect(alias).To(Equal("vEthernet (containerabc)"))
				Expect(mtu).To(Equal(uint32(1302)))
				Expect(family).To(Equal(uint32(windows.AF_INET)))
			})
		})
	})

	Describe("SetNat", func() {
		It("applies the mtu to the NAT network on the host", func() {
			Expect(m.SetNat(1405)).To(Succeed())

			Expect(netInterface.SetMTUCallCount()).To(Equal(1))
			alias, mtu, family := netInterface.SetMTUArgsForCall(0)
			Expect(alias).To(Equal("vEthernet (my-network)"))
			Expect(mtu).To(Equal(uint32(1405)))
			Expect(family).To(Equal(uint32(windows.AF_INET)))
		})

		Context("the specified mtu is 0", func() {
			BeforeEach(func() {
				netInterface.ByIPReturns(netinterface.AdapterInfo{Name: networkName}, nil)
				netInterface.GetMTUReturns(1302, nil)
			})

			It("sets the NAT network MTU to the host interface MTU", func() {
				Expect(m.SetNat(0)).To(Succeed())

				hostIP, err := localip.LocalIP()
				Expect(err).To(Succeed())

				Expect(netInterface.ByIPCallCount()).To(Equal(1))
				Expect(netInterface.ByIPArgsForCall(0)).To(Equal(hostIP))

				Expect(netInterface.GetMTUCallCount()).To(Equal(1))
				alias, family := netInterface.GetMTUArgsForCall(0)
				Expect(alias).To(Equal(networkName))
				Expect(family).To(Equal(uint32(windows.AF_INET)))

				Expect(netInterface.SetMTUCallCount()).To(Equal(1))
				alias, mtu, family := netInterface.SetMTUArgsForCall(0)
				Expect(alias).To(Equal("vEthernet (my-network)"))
				Expect(mtu).To(Equal(uint32(1302)))
				Expect(family).To(Equal(uint32(windows.AF_INET)))
			})
		})
	})
})
