//+build acl

package netrules_test

import (
	"errors"
	"net"

	"code.cloudfoundry.org/winc/network/netrules"
	"code.cloudfoundry.org/winc/network/netrules/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Applier", func() {
	const containerId = "containerabc"
	const containerIP = "5.4.3.2"
	const networkName = "my-network"

	var (
		netSh         *fakes.NetShRunner
		fw            *fakes.Firewall
		portAllocator *fakes.PortAllocator
		netInterface  *fakes.NetInterface
		applier       *netrules.Applier
	)

	BeforeEach(func() {
		netSh = &fakes.NetShRunner{}
		portAllocator = &fakes.PortAllocator{}
		netInterface = &fakes.NetInterface{}
		fw = &fakes.Firewall{}

		applier = netrules.NewApplier(netSh, containerId, networkName, portAllocator, netInterface, fw)
	})

	Describe("In", func() {
		var netInRule netrules.NetIn

		BeforeEach(func() {
			netInRule = netrules.NetIn{
				ContainerPort: 1000,
				HostPort:      2000,
			}
		})

		It("does not create a firewall rule on the host", func() {
			_, _, err := applier.In(netInRule, containerIP)
			Expect(err).NotTo(HaveOccurred())

			Expect(fw.CreateRuleCallCount()).To(Equal(0))
		})
	})

	Describe("Out", func() {
		var (
			protocol   netrules.Protocol
			netOutRule netrules.NetOut
		)

		JustBeforeEach(func() {
			netOutRule = netrules.NetOut{
				Protocol: protocol,
				Networks: []netrules.IPRange{
					ipRangeFromIP(net.ParseIP("8.8.8.8")),
					netrules.IPRange{
						Start: net.ParseIP("10.0.0.0"),
						End:   net.ParseIP("13.0.0.0"),
					},
				},
				Ports: []netrules.PortRange{
					portRangeFromPort(80),
					netrules.PortRange{
						Start: 8080,
						End:   8090,
					},
				},
			}
		})

		Context("a UDP rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolUDP
			})

			It("does not create a firewall rule on the host", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				Expect(fw.CreateRuleCallCount()).To(Equal(0))
			})
		})

		Context("a TCP rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolTCP
			})

			It("does not create a firewall rule on the host", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				Expect(fw.CreateRuleCallCount()).To(Equal(0))
			})
		})

		Context("an ICMP rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolICMP
			})

			It("does not create a firewall rule on the host", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				Expect(fw.CreateRuleCallCount()).To(Equal(0))
			})
		})

		Context("an ANY rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolAll
			})

			It("does not create a firewall rule on the host", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				Expect(fw.CreateRuleCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Cleanup", func() {
		It("does not remove any firewall rules", func() {
			Expect(applier.Cleanup()).To(Succeed())

			Expect(fw.DeleteRuleCallCount()).To(Equal(0))
		})

		Context("releasing ports fails", func() {
			BeforeEach(func() {
				portAllocator.ReleaseAllPortsReturns(errors.New("releasing ports failed"))
			})

			It("does not remove any firewall rules", func() {
				Expect(applier.Cleanup()).To(MatchError("releasing ports failed"))

				Expect(fw.DeleteRuleCallCount()).To(Equal(0))
			})
		})
	})
})
