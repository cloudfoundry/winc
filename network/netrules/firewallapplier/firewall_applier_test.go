package firewallapplier_test

import (
	"errors"
	"net"

	"code.cloudfoundry.org/winc/network/firewall"
	"code.cloudfoundry.org/winc/network/netrules"
	"code.cloudfoundry.org/winc/network/netrules/firewallapplier"
	"code.cloudfoundry.org/winc/network/netrules/firewallapplier/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Firewall Applier", func() {
	const containerId = "containerabc"
	const containerIP = "5.4.3.2"

	var (
		netSh         *fakes.NetShRunner
		fw            *fakes.Firewall
		portAllocator *fakes.PortAllocator
		applier       *firewallapplier.Applier
	)

	BeforeEach(func() {
		netSh = &fakes.NetShRunner{}
		portAllocator = &fakes.PortAllocator{}
		fw = &fakes.Firewall{}

		applier = firewallapplier.NewApplier(netSh, containerId, portAllocator, fw)
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

		It("returns the correct Nat Policy", func() {
			nat, _, err := applier.In(netInRule, containerIP)
			Expect(err).NotTo(HaveOccurred())

			expectedNat := hcsshim.NatPolicy{
				Type:         hcsshim.Nat,
				Protocol:     "TCP",
				InternalPort: 1000,
				ExternalPort: 2000,
			}

			Expect(*nat).To(Equal(expectedNat))
		})

		It("opens the port inside the container", func() {
			_, _, err := applier.In(netInRule, containerIP)
			Expect(err).NotTo(HaveOccurred())

			Expect(netSh.RunContainerCallCount()).To(Equal(1))
			expectedArgs := []string{"http", "add", "urlacl", "url=http://*:1000/", "user=Users"}
			Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedArgs))
		})

		Context("opening the port fails", func() {
			BeforeEach(func() {
				netSh.RunContainerReturns(errors.New("couldn't exec netsh"))
			})

			It("returns an error", func() {
				_, _, err := applier.In(netInRule, containerIP)
				Expect(err).To(MatchError("couldn't exec netsh"))
			})
		})

		Context("the host port is zero", func() {
			BeforeEach(func() {
				netInRule = netrules.NetIn{
					ContainerPort: 1000,
					HostPort:      0,
				}
				portAllocator.AllocatePortReturns(1234, nil)
			})

			It("uses the port allocator to find an open host port", func() {

				nat, _, err := applier.In(netInRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedNat := hcsshim.NatPolicy{
					Type:         hcsshim.Nat,
					Protocol:     "TCP",
					InternalPort: 1000,
					ExternalPort: 1234,
				}

				Expect(*nat).To(Equal(expectedNat))

				Expect(portAllocator.AllocatePortCallCount()).To(Equal(1))
				id, p := portAllocator.AllocatePortArgsForCall(0)
				Expect(id).To(Equal(containerId))
				Expect(p).To(Equal(0))
			})

			Context("when allocating a port fails", func() {
				BeforeEach(func() {
					portAllocator.AllocatePortReturns(0, errors.New("some-error"))
				})

				It("returns an error", func() {
					_, _, err := applier.In(netInRule, containerIP)
					Expect(err).To(MatchError("some-error"))
				})
			})
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

		Context("an invalid protocol is specified", func() {
			BeforeEach(func() {
				protocol = 7
			})

			It("returns an error", func() {
				_, err := applier.Out(netOutRule, containerIP)

				Expect(err).To(MatchError(errors.New("invalid protocol: 7")))
				Expect(fw.CreateRuleCallCount()).To(Equal(0))
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

func ipRangeFromIP(ip net.IP) netrules.IPRange {
	return netrules.IPRange{Start: ip, End: ip}
}

func portRangeFromPort(port uint16) netrules.PortRange {
	return netrules.PortRange{Start: port, End: port}
}
