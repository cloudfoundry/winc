package netrules_test

import (
	"errors"
	"fmt"
	"net"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/netrules"
	"code.cloudfoundry.org/winc/network/netrules/netrulesfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Applier", func() {
	const containerId = "containerabc"
	const containerIP = "5.4.3.2"
	const networkName = "my-network"

	var (
		netSh         *netrulesfakes.FakeNetShRunner
		portAllocator *netrulesfakes.FakePortAllocator
		netInterface  *netrulesfakes.FakeNetInterface
		applier       *netrules.Applier
	)

	BeforeEach(func() {
		netSh = &netrulesfakes.FakeNetShRunner{}
		portAllocator = &netrulesfakes.FakePortAllocator{}
		netInterface = &netrulesfakes.FakeNetInterface{}

		applier = netrules.NewApplier(netSh, containerId, networkName, portAllocator, netInterface)
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
			_, err := applier.In(netInRule, containerIP)
			Expect(err).NotTo(HaveOccurred())

			Expect(netSh.RunHostCallCount()).To(Equal(1))
			expectedArgs := []string{"advfirewall", "firewall", "add", "rule", `name="containerabc"`,
				"dir=in", "action=allow", "localip=5.4.3.2",
				"localport=1000",
				"protocol=TCP"}
			Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedArgs))
		})

		It("returns the correct port mapping", func() {
			mapping, err := applier.In(netInRule, containerIP)
			Expect(err).NotTo(HaveOccurred())

			expectedMapping := netrules.PortMapping{
				ContainerPort: 1000,
				HostPort:      2000,
			}
			Expect(mapping).To(Equal(expectedMapping))
		})

		It("opens the port inside the container", func() {
			_, err := applier.In(netInRule, containerIP)
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
				_, err := applier.In(netInRule, containerIP)
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
				mapping, err := applier.In(netInRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedMapping := netrules.PortMapping{
					ContainerPort: 1000,
					HostPort:      1234,
				}
				Expect(mapping).To(Equal(expectedMapping))

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
					_, err := applier.In(netInRule, containerIP)
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
				Expect(applier.Out(netOutRule, containerIP)).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(1))
				expectedArgs := []string{"advfirewall", "firewall", "add", "rule", `name="containerabc"`,
					"dir=out", "action=allow", "localip=5.4.3.2",
					"remoteip=8.8.8.8-8.8.8.8,10.0.0.0-13.0.0.0",
					"remoteport=80-80,8080-8090",
					"protocol=UDP"}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedArgs))
			})
		})

		Context("a TCP rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolTCP
			})

			It("creates the correct firewall rule on the host", func() {
				Expect(applier.Out(netOutRule, containerIP)).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(1))
				expectedArgs := []string{"advfirewall", "firewall", "add", "rule", `name="containerabc"`,
					"dir=out", "action=allow", "localip=5.4.3.2",
					"remoteip=8.8.8.8-8.8.8.8,10.0.0.0-13.0.0.0",
					"remoteport=80-80,8080-8090",
					"protocol=TCP"}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedArgs))
			})
		})

		Context("an ICMP rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolICMP
			})

			It("creates the correct firewall rule on the host", func() {
				Expect(applier.Out(netOutRule, containerIP)).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(1))
				expectedArgs := []string{"advfirewall", "firewall", "add", "rule", `name="containerabc"`,
					"dir=out", "action=allow", "localip=5.4.3.2",
					"remoteip=8.8.8.8-8.8.8.8,10.0.0.0-13.0.0.0",
					"protocol=ICMPV4"}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedArgs))
			})
		})

		Context("an ANY rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolAll
			})

			It("creates the correct firewall rule on the host", func() {
				Expect(applier.Out(netOutRule, containerIP)).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(1))
				expectedArgs := []string{"advfirewall", "firewall", "add", "rule", `name="containerabc"`,
					"dir=out", "action=allow", "localip=5.4.3.2",
					"remoteip=8.8.8.8-8.8.8.8,10.0.0.0-13.0.0.0",
					"protocol=ANY"}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedArgs))
			})
		})

		Context("an invalid protocol is specified", func() {
			BeforeEach(func() {
				protocol = 7
			})

			It("returns an error", func() {
				Expect(applier.Out(netOutRule, containerIP)).To(MatchError(errors.New("invalid protocol: 7")))
				Expect(netSh.RunHostCallCount()).To(Equal(0))
			})
		})
	})

	Describe("ContainerMTU", func() {
		It("applies the mtu to the container", func() {
			Expect(applier.ContainerMTU(1405)).To(Succeed())

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
				Expect(applier.ContainerMTU(0)).To(Succeed())

				Expect(netInterface.ByNameCallCount()).To(Equal(1))
				Expect(netInterface.ByNameArgsForCall(0)).To(Equal(fmt.Sprintf(`vEthernet (%s)`, networkName)))

				Expect(netInterface.SetMTUCallCount()).To(Equal(1))
				alias, mtu := netInterface.SetMTUArgsForCall(0)
				Expect(alias).To(Equal("vEthernet (containerabc)"))
				Expect(mtu).To(Equal(1302))
			})
		})
	})

	Describe("NatMTU", func() {
		It("applies the mtu to the NAT network on the host", func() {
			Expect(applier.NatMTU(1405)).To(Succeed())

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
				Expect(applier.NatMTU(0)).To(Succeed())

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

	Describe("Cleanup", func() {
		It("removes the firewall rules applied to the container and de-allocates all the ports", func() {
			Expect(applier.Cleanup()).To(Succeed())

			Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
			Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))

			Expect(netSh.RunHostCallCount()).To(Equal(2))
			Expect(netSh.RunHostArgsForCall(0)).To(Equal([]string{"advfirewall", "firewall", "show", "rule", `name="containerabc"`}))
			Expect(netSh.RunHostArgsForCall(1)).To(Equal([]string{"advfirewall", "firewall", "delete", "rule", `name="containerabc"`}))
		})

		Context("there are no firewall rules applied to the container", func() {
			BeforeEach(func() {
				netSh.RunHostReturnsOnCall(0, nil, errors.New("firewall rule not found"))
			})

			It("does not error", func() {
				Expect(applier.Cleanup()).To(Succeed())

				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
				Expect(netSh.RunHostCallCount()).To(Equal(1))
			})
		})

		Context("deleting the firewall rule fails", func() {
			BeforeEach(func() {
				netSh.RunHostReturnsOnCall(1, nil, errors.New("deleting firewall rule failed"))
			})

			It("releases the ports and returns an error", func() {
				Expect(applier.Cleanup()).To(MatchError("deleting firewall rule failed"))

				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
				Expect(netSh.RunHostCallCount()).To(Equal(2))
			})
		})

		Context("releasing ports fails", func() {
			BeforeEach(func() {
				portAllocator.ReleaseAllPortsReturns(errors.New("releasing ports failed"))
			})

			It("still removes firewall rules but returns an error", func() {
				Expect(applier.Cleanup()).To(MatchError("releasing ports failed"))

				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
				Expect(netSh.RunHostCallCount()).To(Equal(2))
			})

			Context("there are no firewall rules applied to the container", func() {
				BeforeEach(func() {
					netSh.RunHostReturnsOnCall(0, nil, errors.New("firewall rule not found"))
				})

				It("returns the port release error", func() {
					Expect(applier.Cleanup()).To(MatchError("releasing ports failed"))

					Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
					Expect(netSh.RunHostCallCount()).To(Equal(1))
				})
			})

			Context("deleting firewall rule also fails", func() {
				BeforeEach(func() {
					netSh.RunHostReturnsOnCall(1, nil, errors.New("deleting firewall rule failed"))
				})

				It("returns a combined error", func() {
					err := applier.Cleanup()
					Expect(err).To(MatchError("releasing ports failed, deleting firewall rule failed"))

					Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
					Expect(netSh.RunHostCallCount()).To(Equal(2))
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
