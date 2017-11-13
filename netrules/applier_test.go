package netrules_test

import (
	"errors"
	"fmt"
	"net"

	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/netrules/netrulesfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Applier", func() {
	const containerId = "containerabc"
	const endpointId = "1234-endpoint"
	const networkName = "my-network"

	var (
		netSh         *netrulesfakes.FakeNetShRunner
		portAllocator *netrulesfakes.FakePortAllocator
		applier       *netrules.Applier
	)

	BeforeEach(func() {
		netSh = &netrulesfakes.FakeNetShRunner{}
		portAllocator = &netrulesfakes.FakePortAllocator{}

		applier = netrules.NewApplier(netSh, containerId, networkName, portAllocator)
	})

	Describe("In", func() {
		var netInRule netrules.NetIn

		BeforeEach(func() {
			netInRule = netrules.NetIn{
				ContainerPort: 1000,
				HostPort:      2000,
			}
		})

		It("returns the correct NAT policy + ACL policy", func() {
			natPolicy, aclPolicy, err := applier.In(netInRule)
			Expect(err).NotTo(HaveOccurred())

			expectedNatPolicy := hcsshim.NatPolicy{
				Type:         hcsshim.Nat,
				InternalPort: 1000,
				ExternalPort: 2000,
				Protocol:     "TCP",
			}
			expectedAclPolicy := hcsshim.ACLPolicy{
				Type:      hcsshim.ACL,
				Protocol:  6,
				Action:    hcsshim.Allow,
				Direction: hcsshim.In,
				LocalPort: 1000,
			}
			Expect(*natPolicy).To(Equal(expectedNatPolicy))
			Expect(*aclPolicy).To(Equal(expectedAclPolicy))
		})

		It("opens the port inside the container", func() {
			_, _, err := applier.In(netInRule)
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
				_, _, err := applier.In(netInRule)
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
				natPolicy, aclPolicy, err := applier.In(netInRule)
				Expect(err).NotTo(HaveOccurred())

				expectedNatPolicy := hcsshim.NatPolicy{
					Type:         hcsshim.Nat,
					InternalPort: 1000,
					ExternalPort: 1234,
					Protocol:     "TCP",
				}
				expectedAclPolicy := hcsshim.ACLPolicy{
					Type:      hcsshim.ACL,
					Protocol:  6,
					Action:    hcsshim.Allow,
					Direction: hcsshim.In,
					LocalPort: 1000,
				}
				Expect(*natPolicy).To(Equal(expectedNatPolicy))
				Expect(*aclPolicy).To(Equal(expectedAclPolicy))

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
					_, _, err := applier.In(netInRule)
					Expect(err).To(MatchError("some-error"))
				})
			})
		})
	})

	Describe("Out", func() {
		var (
			netOutRule netrules.NetOut
		)

		BeforeEach(func() {
			netOutRule = netrules.NetOut{
				Protocol: netrules.ProtocolTCP,
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
						End:   8081,
					},
				},
			}
		})

		It("returns an ACL Policy per port + IP CIDR", func() {
			expectedACLPolicies := []hcsshim.ACLPolicy{
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 80, RemoteAddresses: "8.8.8.8/32"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 80, RemoteAddresses: "10.0.0.0/7"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 80, RemoteAddresses: "12.0.0.0/8"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 80, RemoteAddresses: "13.0.0.0/32"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8080, RemoteAddresses: "8.8.8.8/32"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8080, RemoteAddresses: "10.0.0.0/7"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8080, RemoteAddresses: "12.0.0.0/8"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8080, RemoteAddresses: "13.0.0.0/32"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8081, RemoteAddresses: "8.8.8.8/32"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8081, RemoteAddresses: "10.0.0.0/7"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8081, RemoteAddresses: "12.0.0.0/8"},
				{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8081, RemoteAddresses: "13.0.0.0/32"},
			}

			aclPolicies, err := applier.Out(netOutRule)
			Expect(err).To(Succeed())

			Expect(len(aclPolicies)).To(Equal(12))
			acls := []hcsshim.ACLPolicy{}
			for _, acl := range aclPolicies {
				acls = append(acls, *acl)
			}
			Expect(acls).To(ConsistOf(expectedACLPolicies))
		})

		Context("a UDP rule is specified", func() {
			BeforeEach(func() {
				netOutRule.Networks = []netrules.IPRange{
					{Start: net.ParseIP("1.1.1.1"), End: net.ParseIP("1.1.1.1")},
				}
				netOutRule.Ports = []netrules.PortRange{{Start: 5, End: 5}}
				netOutRule.Protocol = netrules.ProtocolUDP
			})

			It("uses UDP for each policy", func() {
				aclPolicies, err := applier.Out(netOutRule)
				Expect(err).To(Succeed())

				expectedACLPolicies := []hcsshim.ACLPolicy{
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolUDP, RemotePort: 5, RemoteAddresses: "1.1.1.1/32"},
				}

				Expect(len(aclPolicies)).To(Equal(1))
				acls := []hcsshim.ACLPolicy{}
				for _, acl := range aclPolicies {
					acls = append(acls, *acl)
				}
				Expect(acls).To(ConsistOf(expectedACLPolicies))
			})
		})

		Context("a TCP rule is specified", func() {
			BeforeEach(func() {
				netOutRule.Networks = []netrules.IPRange{
					{Start: net.ParseIP("1.1.1.1"), End: net.ParseIP("1.1.1.1")},
				}
				netOutRule.Ports = []netrules.PortRange{{Start: 5, End: 5}}
				netOutRule.Protocol = netrules.ProtocolTCP
			})

			It("uses TCP for each policy", func() {
				aclPolicies, err := applier.Out(netOutRule)
				Expect(err).To(Succeed())

				expectedACLPolicies := []hcsshim.ACLPolicy{
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 5, RemoteAddresses: "1.1.1.1/32"},
				}

				Expect(len(aclPolicies)).To(Equal(1))
				acls := []hcsshim.ACLPolicy{}
				for _, acl := range aclPolicies {
					acls = append(acls, *acl)
				}
				Expect(acls).To(ConsistOf(expectedACLPolicies))
			})
		})

		Context("an ICMP rule is specified", func() {
			BeforeEach(func() {
				netOutRule.Networks = []netrules.IPRange{
					{Start: net.ParseIP("1.1.1.1"), End: net.ParseIP("1.1.1.1")},
				}
				netOutRule.Ports = []netrules.PortRange{{Start: 5, End: 5}}
				netOutRule.Protocol = netrules.ProtocolICMP
			})

			It("uses ICMP for each policy, making sure not to set the por", func() {
				aclPolicies, err := applier.Out(netOutRule)
				Expect(err).To(Succeed())

				expectedACLPolicies := []hcsshim.ACLPolicy{
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolICMP, RemoteAddresses: "1.1.1.1/32"},
				}

				Expect(len(aclPolicies)).To(Equal(1))
				acls := []hcsshim.ACLPolicy{}
				for _, acl := range aclPolicies {
					acls = append(acls, *acl)
				}
				Expect(acls).To(ConsistOf(expectedACLPolicies))
			})
		})

		Context("an ANY rule is specified", func() {
			BeforeEach(func() {
				netOutRule.Networks = []netrules.IPRange{
					{Start: net.ParseIP("1.1.1.1"), End: net.ParseIP("1.1.1.1")},
				}
				netOutRule.Ports = []netrules.PortRange{{Start: 5, End: 5}}
				netOutRule.Protocol = netrules.ProtocolAll
			})

			It("creates a rule for each protocol", func() {
				aclPolicies, err := applier.Out(netOutRule)
				Expect(err).To(Succeed())

				expectedACLPolicies := []hcsshim.ACLPolicy{
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 5, RemoteAddresses: "1.1.1.1/32"},
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolICMP, RemoteAddresses: "1.1.1.1/32"},
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolUDP, RemotePort: 5, RemoteAddresses: "1.1.1.1/32"},
				}

				Expect(len(aclPolicies)).To(Equal(3))
				acls := []hcsshim.ACLPolicy{}
				for _, acl := range aclPolicies {
					acls = append(acls, *acl)
				}
				Expect(acls).To(ConsistOf(expectedACLPolicies))
			})
		})

		Context("an invalid protocol is specified", func() {
			BeforeEach(func() {
				netOutRule.Protocol = 255
			})

			It("returns an error", func() {
				_, err := applier.Out(netOutRule)
				Expect(err).To(MatchError(errors.New("invalid protocol: 255")))
			})
		})

		Context("no ports are supplied", func() {
			BeforeEach(func() {
				netOutRule = netrules.NetOut{
					Protocol: netrules.ProtocolTCP,
					Networks: []netrules.IPRange{
						netrules.IPRange{
							Start: net.ParseIP("10.0.0.0"),
							End:   net.ParseIP("13.0.0.0"),
						},
					},
					Ports: []netrules.PortRange{},
				}
			})

			It("returns acls with no port specified", func() {
				expectedACLPolicies := []hcsshim.ACLPolicy{
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemoteAddresses: "10.0.0.0/7"},
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemoteAddresses: "12.0.0.0/8"},
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemoteAddresses: "13.0.0.0/32"},
				}

				aclPolicies, err := applier.Out(netOutRule)
				Expect(err).To(Succeed())

				Expect(len(aclPolicies)).To(Equal(3))
				acls := []hcsshim.ACLPolicy{}
				for _, acl := range aclPolicies {
					acls = append(acls, *acl)
				}
				Expect(acls).To(ConsistOf(expectedACLPolicies))
			})
		})

		Context("no ips are supplied", func() {
			BeforeEach(func() {
				netOutRule = netrules.NetOut{
					Protocol: netrules.ProtocolTCP,
					Ports: []netrules.PortRange{
						portRangeFromPort(80),
						netrules.PortRange{
							Start: 8080,
							End:   8081,
						},
					},
				}
			})

			It("returns acls with empty RemoteAddresses", func() {
				expectedACLPolicies := []hcsshim.ACLPolicy{
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 80},
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8080},
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP, RemotePort: 8081},
				}

				aclPolicies, err := applier.Out(netOutRule)
				Expect(err).To(Succeed())

				Expect(len(aclPolicies)).To(Equal(3))
				acls := []hcsshim.ACLPolicy{}
				for _, acl := range aclPolicies {
					acls = append(acls, *acl)
				}
				Expect(acls).To(ConsistOf(expectedACLPolicies))
			})
		})

		Context("no ports and no ips are supplied", func() {
			BeforeEach(func() {
				netOutRule = netrules.NetOut{
					Protocol: netrules.ProtocolTCP,
				}
			})

			It("returns an acl that allows everything", func() {
				expectedACLPolicies := []hcsshim.ACLPolicy{
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Allow, Protocol: netrules.WindowsProtocolTCP},
				}

				aclPolicies, err := applier.Out(netOutRule)
				Expect(err).To(Succeed())

				Expect(len(aclPolicies)).To(Equal(1))
				acls := []hcsshim.ACLPolicy{}
				for _, acl := range aclPolicies {
					acls = append(acls, *acl)
				}
				Expect(acls).To(ConsistOf(expectedACLPolicies))
			})
		})
	})

	Describe("ContainerMTU", func() {
		It("applies the mtu to the container", func() {
			Expect(applier.ContainerMTU(1405)).To(Succeed())

			Expect(netSh.RunContainerCallCount()).To(Equal(1))
			expectedMTUArgs := []string{"interface", "ipv4", "set", "subinterface",
				`"vEthernet (containerabc)"`, "mtu=1405", "store=persistent"}
			Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedMTUArgs))
		})

		Context("the specified mtu is 0", func() {
			BeforeEach(func() {
				netSh.RunHostReturns([]byte(`
MTU  MediaSenseState   Bytes In  Bytes Out  Interface
------  ---------------  ---------  ---------  -------------
1302                1     142864    2448382  vEthernet (my-network)
			`), nil)
			})

			It("sets the container MTU to the NAT network MTU", func() {
				Expect(applier.ContainerMTU(0)).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(1))
				expectedMTUArgs := []string{"interface", "ipv4", "show", "subinterface",
					fmt.Sprintf("interface=vEthernet (%s)", networkName)}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedMTUArgs))

				Expect(netSh.RunContainerCallCount()).To(Equal(1))
				expectedMTUArgs = []string{"interface", "ipv4", "set", "subinterface",
					fmt.Sprintf(`"vEthernet (%s)"`, containerId), "mtu=1302", "store=persistent"}
				Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedMTUArgs))
			})
		})

		Context("the specified mtu is > 1500", func() {
			It("returns an error", func() {
				err := applier.ContainerMTU(1600)
				Expect(err).To(MatchError(errors.New("invalid mtu specified: 1600")))
				Expect(netSh.RunContainerCallCount()).To(Equal(0))
			})
		})
	})

	Describe("NatMTU", func() {
		It("applies the mtu to the NAT network on the host", func() {
			Expect(applier.NatMTU(1405)).To(Succeed())

			Expect(netSh.RunHostCallCount()).To(Equal(1))
			expectedMTUArgs := []string{"interface", "ipv4", "set", "subinterface",
				`vEthernet (my-network)`, "mtu=1405", "store=persistent"}
			Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedMTUArgs))
		})

		Context("the specified mtu is 0", func() {
			BeforeEach(func() {
				netSh.RunHostReturnsOnCall(0, []byte(`
MTU  MediaSenseState   Bytes In  Bytes Out  Interface
------  ---------------  ---------  ---------  -------------
1302                1     142864    2448382  Ethernet
			`), nil)
			})

			It("sets the NAT network MTU to the host interface MTU", func() {
				Expect(applier.NatMTU(0)).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(2))
				expectedMTUArgs := []string{"interface", "ipv4", "show", "subinterface",
					"interface=Ethernet"}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedMTUArgs))

				expectedMTUArgs = []string{"interface", "ipv4", "set", "subinterface",
					fmt.Sprintf(`vEthernet (%s)`, networkName), "mtu=1302", "store=persistent"}
				Expect(netSh.RunHostArgsForCall(1)).To(Equal(expectedMTUArgs))
			})
		})

		Context("the specified mtu is > 1500", func() {
			It("returns an error", func() {
				err := applier.NatMTU(1600)
				Expect(err).To(MatchError(errors.New("invalid mtu specified: 1600")))
				Expect(netSh.RunHostCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Cleanup", func() {
		It("de-allocates all the ports", func() {
			Expect(applier.Cleanup()).To(Succeed())

			Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
			Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))
		})
	})
})

func ipRangeFromIP(ip net.IP) netrules.IPRange {
	return netrules.IPRange{Start: ip, End: ip}
}

func portRangeFromPort(port uint16) netrules.PortRange {
	return netrules.PortRange{Start: port, End: port}
}
