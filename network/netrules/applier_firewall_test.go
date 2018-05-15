//+build !acl

package netrules_test

import (
	"errors"
	"net"

	"code.cloudfoundry.org/winc/network/firewall"
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

		It("creates the correct firewall rule on the host", func() {
			_, _, err := applier.In(netInRule, containerIP)
			Expect(err).NotTo(HaveOccurred())

			expectedRule := firewall.Rule{
				Name:           "containerabc",
				Direction:      firewall.NET_FW_RULE_DIR_IN,
				Action:         firewall.NET_FW_ACTION_ALLOW,
				LocalAddresses: "5.4.3.2",
				LocalPorts:     "1000",
				Protocol:       firewall.NET_FW_IP_PROTOCOL_TCP,
			}

			Expect(fw.CreateRuleCallCount()).To(Equal(1))
			Expect(fw.CreateRuleArgsForCall(0)).To(Equal(expectedRule))
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

			It("creates the correct firewall rule on the host", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedRule := firewall.Rule{
					Name:            "containerabc",
					Direction:       firewall.NET_FW_RULE_DIR_OUT,
					Action:          firewall.NET_FW_ACTION_ALLOW,
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "8.8.8.8-8.8.8.8,10.0.0.0-13.0.0.0",
					RemotePorts:     "80-80,8080-8090",
					Protocol:        firewall.NET_FW_IP_PROTOCOL_UDP,
				}

				Expect(fw.CreateRuleCallCount()).To(Equal(1))
				Expect(fw.CreateRuleArgsForCall(0)).To(Equal(expectedRule))
			})
		})

		Context("a TCP rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolTCP
			})

			It("creates the correct firewall rule on the host", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedRule := firewall.Rule{
					Name:            "containerabc",
					Direction:       firewall.NET_FW_RULE_DIR_OUT,
					Action:          firewall.NET_FW_ACTION_ALLOW,
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "8.8.8.8-8.8.8.8,10.0.0.0-13.0.0.0",
					RemotePorts:     "80-80,8080-8090",
					Protocol:        firewall.NET_FW_IP_PROTOCOL_TCP,
				}

				Expect(fw.CreateRuleCallCount()).To(Equal(1))
				Expect(fw.CreateRuleArgsForCall(0)).To(Equal(expectedRule))
			})
		})

		Context("an ICMP rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolICMP
			})

			It("creates the correct firewall rule on the host", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedRule := firewall.Rule{
					Name:            "containerabc",
					Direction:       firewall.NET_FW_RULE_DIR_OUT,
					Action:          firewall.NET_FW_ACTION_ALLOW,
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "8.8.8.8-8.8.8.8,10.0.0.0-13.0.0.0",
					Protocol:        firewall.NET_FW_IP_PROTOCOL_ICMP,
				}

				Expect(fw.CreateRuleCallCount()).To(Equal(1))
				Expect(fw.CreateRuleArgsForCall(0)).To(Equal(expectedRule))
			})
		})

		Context("an ANY rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolAll
			})

			It("creates the correct firewall rule on the host", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedRule := firewall.Rule{
					Name:            "containerabc",
					Direction:       firewall.NET_FW_RULE_DIR_OUT,
					Action:          firewall.NET_FW_ACTION_ALLOW,
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "8.8.8.8-8.8.8.8,10.0.0.0-13.0.0.0",
					Protocol:        firewall.NET_FW_IP_PROTOCOL_ANY,
				}

				Expect(fw.CreateRuleCallCount()).To(Equal(1))
				Expect(fw.CreateRuleArgsForCall(0)).To(Equal(expectedRule))
			})
		})
	})

	Describe("Cleanup", func() {
		It("removes the firewall rules applied to the container and de-allocates all the ports", func() {
			Expect(applier.Cleanup()).To(Succeed())

			Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
			Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))

			Expect(fw.DeleteRuleCallCount()).To(Equal(1))
			Expect(fw.DeleteRuleArgsForCall(0)).To(Equal("containerabc"))
		})

		Context("deleting the firewall rule fails", func() {
			BeforeEach(func() {
				fw.DeleteRuleReturnsOnCall(0, errors.New("deleting firewall rule failed"))
			})

			It("releases the ports and returns an error", func() {
				Expect(applier.Cleanup()).To(MatchError("deleting firewall rule failed"))

				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
			})
		})

		Context("releasing ports fails", func() {
			BeforeEach(func() {
				portAllocator.ReleaseAllPortsReturns(errors.New("releasing ports failed"))
			})

			It("still removes firewall rules but returns an error", func() {
				Expect(applier.Cleanup()).To(MatchError("releasing ports failed"))

				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
				Expect(fw.DeleteRuleCallCount()).To(Equal(1))
			})

			Context("deleting firewall rule also fails", func() {
				BeforeEach(func() {
					fw.DeleteRuleReturnsOnCall(0, errors.New("deleting firewall rule failed"))
				})

				It("returns a combined error", func() {
					err := applier.Cleanup()
					Expect(err).To(MatchError("releasing ports failed, deleting firewall rule failed"))

					Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
					Expect(fw.DeleteRuleCallCount()).To(Equal(1))
				})
			})
		})
	})
})
