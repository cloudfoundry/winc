package mtu_test

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/mtu"
	"code.cloudfoundry.org/winc/network/mtu/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			alias, mtu := netInterface.SetMTUArgsForCall(0)
			Expect(alias).To(Equal("vEthernet (containerabc)"))
			Expect(mtu).To(Equal(1405))
		})

		Context("the specified mtu is 0", func() {
			BeforeEach(func() {
				netInterface.ByNameReturns(&net.Interface{MTU: 1302}, nil)
			})

			It("sets the container MTU to the NAT network MTU", func() {
				Expect(m.SetContainer(0)).To(Succeed())

				Expect(netInterface.ByNameCallCount()).To(Equal(1))
				Expect(netInterface.ByNameArgsForCall(0)).To(Equal(fmt.Sprintf(`vEthernet (%s)`, networkName)))

				Expect(netInterface.SetMTUCallCount()).To(Equal(1))
				alias, mtu := netInterface.SetMTUArgsForCall(0)
				Expect(alias).To(Equal("vEthernet (containerabc)"))
				Expect(mtu).To(Equal(1302))
			})
		})
	})

	Describe("SetNat", func() {
		It("applies the mtu to the NAT network on the host", func() {
			Expect(m.SetNat(1405)).To(Succeed())

			Expect(netInterface.SetMTUCallCount()).To(Equal(1))
			alias, mtu := netInterface.SetMTUArgsForCall(0)
			Expect(alias).To(Equal("vEthernet (my-network)"))
			Expect(mtu).To(Equal(1405))
		})

		Context("the specified mtu is 0", func() {
			BeforeEach(func() {
				netInterface.ByIPReturns(&net.Interface{MTU: 1302}, nil)
			})

			It("sets the NAT network MTU to the host interface MTU", func() {
				Expect(m.SetNat(0)).To(Succeed())

				hostIP, err := localip.LocalIP()
				Expect(err).To(Succeed())

				Expect(netInterface.ByIPCallCount()).To(Equal(1))
				Expect(netInterface.ByIPArgsForCall(0)).To(Equal(hostIP))

				Expect(netInterface.SetMTUCallCount()).To(Equal(1))
				alias, mtu := netInterface.SetMTUArgsForCall(0)
				Expect(alias).To(Equal("vEthernet (my-network)"))
				Expect(mtu).To(Equal(1302))
			})
		})
	})
})
