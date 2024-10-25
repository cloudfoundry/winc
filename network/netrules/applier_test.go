package netrules_test

import (
	"errors"
	"fmt"
	"net"

	"code.cloudfoundry.org/winc/network/firewall"
	"code.cloudfoundry.org/winc/network/netrules"
	"code.cloudfoundry.org/winc/network/netrules/fakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Applier", func() {
	const containerId = "containerabc"
	const containerIP = "5.4.3.2"

	var (
		netSh         *fakes.NetShRunner
		portAllocator *fakes.PortAllocator
		applier       *netrules.Applier
	)

	BeforeEach(func() {
		netSh = &fakes.NetShRunner{}
		portAllocator = &fakes.PortAllocator{}

		applier = netrules.NewApplier(netSh, containerId, portAllocator)
	})

	Describe("OpenPort", func() {

		var port uint32

		BeforeEach(func() {
			port = 999
		})
		It("opens the port inside the container", func() {
			err := applier.OpenPort(port)
			Expect(err).NotTo(HaveOccurred())

			Expect(netSh.RunContainerCallCount()).To(Equal(1))
			expectedArgs := []string{"http", "add", "urlacl", fmt.Sprintf("url=http://*:%d/", port), "user=Users"}
			Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedArgs))
		})

		Context("opening the port fails", func() {
			BeforeEach(func() {
				netSh.RunContainerReturns(errors.New("couldn't exec netsh"))
			})

			It("returns an error", func() {
				err := applier.OpenPort(port)
				Expect(err).To(MatchError("couldn't exec netsh"))
			})
		})
	})

	Describe("In", func() {
		var netInRule netrules.NetIn

		BeforeEach(func() {
			netInRule = netrules.NetIn{
				ContainerPort: 1000,
				HostPort:      2000,
			}
		})

		It("returns the correct nat and acl policies", func() {
			nat, acl, err := applier.In(netInRule, containerIP)
			Expect(err).NotTo(HaveOccurred())

			expectedNat := hcsshim.NatPolicy{
				Type:         hcsshim.Nat,
				Protocol:     "TCP",
				InternalPort: 1000,
				ExternalPort: 2000,
			}
			Expect(*nat).To(Equal(expectedNat))

			expectedAcl := hcsshim.ACLPolicy{
				Type:           hcsshim.ACL,
				Action:         hcsshim.Allow,
				Direction:      hcsshim.In,
				Protocol:       6,
				LocalAddresses: "5.4.3.2",
				LocalPorts:     "1000",
			}
			Expect(*acl).To(Equal(expectedAcl))
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
				nat, acl, err := applier.In(netInRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedNat := hcsshim.NatPolicy{
					Type:         hcsshim.Nat,
					Protocol:     "TCP",
					InternalPort: 1000,
					ExternalPort: 1234,
				}
				Expect(*nat).To(Equal(expectedNat))

				expectedAcl := hcsshim.ACLPolicy{
					Type:           hcsshim.ACL,
					Action:         hcsshim.Allow,
					Direction:      hcsshim.In,
					Protocol:       6,
					LocalAddresses: "5.4.3.2",
					LocalPorts:     "1000",
				}
				Expect(*acl).To(Equal(expectedAcl))

				Expect(portAllocator.AllocatePortCallCount()).To(Equal(1))
				id, p := portAllocator.AllocatePortArgsForCall(0)
				Expect(id).To(Equal(containerId))
				Expect(p).To(Equal(uint16(0)))
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
		var netOutRule netrules.NetOut

		BeforeEach(func() {
			netOutRule = netrules.NetOut{
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
				netOutRule.Protocol = netrules.ProtocolUDP
			})

			It("returns the correct HNS ACL", func() {
				acl, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedAcl := hcsshim.ACLPolicy{
					Type:            hcsshim.ACL,
					Action:          hcsshim.Allow,
					Direction:       hcsshim.Out,
					Protocol:        uint16(firewall.NET_FW_IP_PROTOCOL_UDP),
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "8.8.8.8/32,10.0.0.0/7,12.0.0.0/8,13.0.0.0/32",
					RemotePorts:     "80-80,8080-8090",
				}
				Expect(*acl).To(Equal(expectedAcl))
			})
		})

		Context("a TCP rule is specified", func() {
			BeforeEach(func() {
				netOutRule.Protocol = netrules.ProtocolTCP
			})

			It("returns the correct HNS ACL", func() {
				acl, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedAcl := hcsshim.ACLPolicy{
					Type:            hcsshim.ACL,
					Action:          hcsshim.Allow,
					Direction:       hcsshim.Out,
					Protocol:        uint16(firewall.NET_FW_IP_PROTOCOL_TCP),
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "8.8.8.8/32,10.0.0.0/7,12.0.0.0/8,13.0.0.0/32",
					RemotePorts:     "80-80,8080-8090",
				}
				Expect(*acl).To(Equal(expectedAcl))
			})
		})

		Context("an ICMP rule is specified", func() {
			BeforeEach(func() {
				netOutRule.Protocol = netrules.ProtocolICMP
			})

			It("returns the correct HNS ACL", func() {
				acl, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedAcl := hcsshim.ACLPolicy{
					Type:            hcsshim.ACL,
					Action:          hcsshim.Allow,
					Direction:       hcsshim.Out,
					Protocol:        uint16(firewall.NET_FW_IP_PROTOCOL_ICMP),
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "8.8.8.8/32,10.0.0.0/7,12.0.0.0/8,13.0.0.0/32",
				}
				Expect(*acl).To(Equal(expectedAcl))
			})
		})

		Context("an ANY rule is specified", func() {
			BeforeEach(func() {
				netOutRule.Protocol = netrules.ProtocolAll
			})

			It("returns the correct HNS ACL", func() {
				acl, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedAcl := hcsshim.ACLPolicy{
					Type:            hcsshim.ACL,
					Action:          hcsshim.Allow,
					Direction:       hcsshim.Out,
					Protocol:        uint16(firewall.NET_FW_IP_PROTOCOL_ANY),
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "8.8.8.8/32,10.0.0.0/7,12.0.0.0/8,13.0.0.0/32",
				}
				Expect(*acl).To(Equal(expectedAcl))
			})
		})

		Context("netout contains an ip range that resolves to 0.0.0.0/0", func() {
			BeforeEach(func() {
				netOutRule.Networks = []netrules.IPRange{
					ipRangeFromIP(net.ParseIP("8.8.8.8")),
					netrules.IPRange{
						Start: net.ParseIP("0.0.0.0"),
						End:   net.ParseIP("255.255.255.255"),
					},
				}
				netOutRule.Protocol = netrules.ProtocolAll
			})

			It("returns an HNS ACL with empty remote addresses", func() {
				acl, err := applier.Out(netOutRule, containerIP)
				Expect(err).NotTo(HaveOccurred())

				expectedAcl := hcsshim.ACLPolicy{
					Type:            hcsshim.ACL,
					Action:          hcsshim.Allow,
					Direction:       hcsshim.Out,
					Protocol:        uint16(firewall.NET_FW_IP_PROTOCOL_ANY),
					LocalAddresses:  "5.4.3.2",
					RemoteAddresses: "",
				}
				Expect(*acl).To(Equal(expectedAcl))
			})
		})

		Context("an invalid protocol is specified", func() {
			BeforeEach(func() {
				netOutRule.Protocol = 7
			})

			It("returns an error", func() {
				_, err := applier.Out(netOutRule, containerIP)
				Expect(err).To(MatchError(errors.New("invalid protocol: 7")))
			})
		})
	})

	Describe("Cleanup", func() {
		It("de-allocates all the ports", func() {
			Expect(applier.Cleanup()).To(Succeed())

			Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
			Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))
		})

		Context("releasing ports fails", func() {
			BeforeEach(func() {
				portAllocator.ReleaseAllPortsReturns(errors.New("releasing ports failed"))
			})

			It("returns the error", func() {
				Expect(applier.Cleanup()).To(MatchError("releasing ports failed"))

				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
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
