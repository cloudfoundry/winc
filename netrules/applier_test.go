package netrules_test

import (
	"encoding/json"
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

	var (
		netSh    *netrulesfakes.FakeNetShRunner
		applier  *netrules.Applier
		endpoint *hcsshim.HNSEndpoint
	)

	BeforeEach(func() {
		endpoint = &hcsshim.HNSEndpoint{}

		netSh = &netrulesfakes.FakeNetShRunner{}
		applier = netrules.NewApplier(netSh, containerId, false, "")
	})

	Describe("In", func() {
		var (
			netInRule     netrules.NetIn
			containerPort uint32
			hostPort      uint32
		)

		BeforeEach(func() {
			policyOne := hcsshim.NatPolicy{
				Type:         "NAT",
				Protocol:     "TCP",
				InternalPort: 8080,
				ExternalPort: 1234,
			}
			policyOneJSON, err := json.Marshal(policyOne)
			Expect(err).NotTo(HaveOccurred())

			policyTwo := hcsshim.NatPolicy{
				Type:         "NAT",
				Protocol:     "TCP",
				InternalPort: 2222,
				ExternalPort: 9876,
			}
			policyTwoJSON, err := json.Marshal(policyTwo)
			Expect(err).NotTo(HaveOccurred())

			endpoint.Policies = []json.RawMessage{policyOneJSON, policyTwoJSON}
		})

		JustBeforeEach(func() {
			netInRule = netrules.NetIn{
				ContainerPort: containerPort,
				HostPort:      hostPort,
			}
		})

		Context("the container port is 8080", func() {
			BeforeEach(func() {
				containerPort = 8080
				hostPort = 0
			})

			It("returns the associated mapping from the endpoint", func() {
				portMapping, err := applier.In(netInRule, endpoint)
				Expect(err).ToNot(HaveOccurred())
				Expect(portMapping).To(Equal(netrules.PortMapping{
					ContainerPort: 8080,
					HostPort:      1234,
				}))
			})

			It("applies a urlacl to the specified port in the container", func() {
				_, err := applier.In(netInRule, endpoint)
				Expect(err).ToNot(HaveOccurred())

				Expect(netSh.RunContainerCallCount()).To(Equal(1))
				expectedArgs := []string{"http", "add", "urlacl", "url=http://*:8080/", "user=Users"}
				Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedArgs))
			})
		})

		Context("the container port is 2222", func() {
			BeforeEach(func() {
				containerPort = 2222
				hostPort = 0
			})

			It("returns the associated mapping from the endpoint", func() {
				portMapping, err := applier.In(netInRule, endpoint)
				Expect(err).ToNot(HaveOccurred())
				Expect(portMapping).To(Equal(netrules.PortMapping{
					ContainerPort: 2222,
					HostPort:      9876,
				}))
			})

			It("applies a urlacl to the specified port in the container", func() {
				_, err := applier.In(netInRule, endpoint)
				Expect(err).ToNot(HaveOccurred())

				Expect(netSh.RunContainerCallCount()).To(Equal(1))
				expectedArgs := []string{"http", "add", "urlacl", "url=http://*:2222/", "user=Users"}
				Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedArgs))
			})
		})

		Context("the container port is not 8080 or 2222", func() {
			BeforeEach(func() {
				containerPort = 1234
				hostPort = 0
			})

			It("returns an error", func() {
				_, err := applier.In(netInRule, endpoint)
				Expect(err).To(MatchError(errors.New("invalid port mapping: host 0, container 1234")))
				Expect(netSh.RunContainerCallCount()).To(Equal(0))
			})
		})

		Context("the host port is not 0", func() {
			BeforeEach(func() {
				containerPort = 8080
				hostPort = 1234
			})

			It("returns an error", func() {
				_, err := applier.In(netInRule, endpoint)
				Expect(err).To(MatchError(errors.New("invalid port mapping: host 1234, container 8080")))
				Expect(netSh.RunContainerCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Out", func() {
		var (
			protocol   netrules.Protocol
			netOutRule netrules.NetOut
		)

		BeforeEach(func() {
			endpoint.IPAddress = net.ParseIP("5.4.3.2")
		})

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
				Expect(applier.Out(netOutRule, endpoint)).To(Succeed())

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
				Expect(applier.Out(netOutRule, endpoint)).To(Succeed())

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

			It("ignores it", func() {
				Expect(applier.Out(netOutRule, endpoint)).To(Succeed())
				Expect(netSh.RunHostCallCount()).To(Equal(0))
			})
		})

		Context("an ANY rule is specified", func() {
			BeforeEach(func() {
				protocol = netrules.ProtocolAll
			})

			It("creates the correct firewall rule on the host", func() {
				Expect(applier.Out(netOutRule, endpoint)).To(Succeed())

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
				Expect(applier.Out(netOutRule, endpoint)).To(MatchError(errors.New("invalid protocol: 7")))
				Expect(netSh.RunHostCallCount()).To(Equal(0))
			})
		})
	})

	Describe("MTU", func() {
		It("applies the mtu to the container", func() {
			Expect(applier.MTU(endpointId, 1405)).To(Succeed())

			Expect(netSh.RunContainerCallCount()).To(Equal(1))
			expectedMTUArgs := []string{"interface", "ipv4", "set", "subinterface",
				`"vEthernet (Container NIC 1234)"`, "mtu=1405", "store=persistent"}
			Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedMTUArgs))
		})

		Context("the specified mtu is 0", func() {
			BeforeEach(func() {
				netSh.RunHostReturns([]byte(`
   MTU  MediaSenseState   Bytes In  Bytes Out  Interface
------  ---------------  ---------  ---------  -------------
  1302                1     142864    2448382  vEthernet (HNS Internal NIC)
				`), nil)
			})

			It("sets the container MTU to the host MTU", func() {
				Expect(applier.MTU(endpointId, 0)).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(1))
				expectedMTUArgs := []string{"interface", "ipv4", "show", "subinterface",
					"interface=vEthernet (HNS Internal NIC)"}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedMTUArgs))

				Expect(netSh.RunContainerCallCount()).To(Equal(1))
				expectedMTUArgs = []string{"interface", "ipv4", "set", "subinterface",
					`"vEthernet (Container NIC 1234)"`, "mtu=1302", "store=persistent"}
				Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedMTUArgs))
			})
		})

		Context("the specified mtu is > 1500", func() {
			It("returns an error", func() {
				err := applier.MTU(endpointId, 1600)
				Expect(err).To(MatchError(errors.New("invalid mtu specified: 1600")))
				Expect(netSh.RunContainerCallCount()).To(Equal(0))
			})
		})

		Context("when run on a technical preview", func() {
			BeforeEach(func() {
				applier = netrules.NewApplier(netSh, containerId, true, "some-network-name")

				netSh.RunHostReturns([]byte(`
   MTU  MediaSenseState   Bytes In  Bytes Out  Interface
------  ---------------  ---------  ---------  -------------
  1302                1     142864    2448382  vEthernet (some-network-name)
				`), nil)
			})

			It("uses the correct interface names", func() {
				interfaceName := "some-interface-name"
				Expect(applier.MTU(interfaceName, 0)).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(1))
				expectedMTUArgs := []string{"interface", "ipv4", "show", "subinterface",
					"interface=vEthernet (some-network-name)"}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedMTUArgs))

				Expect(netSh.RunContainerCallCount()).To(Equal(1))
				expectedMTUArgs = []string{"interface", "ipv4", "set", "subinterface",
					fmt.Sprintf(`"vEthernet (%s)"`, interfaceName), "mtu=1302", "store=persistent"}
				Expect(netSh.RunContainerArgsForCall(0)).To(Equal(expectedMTUArgs))
			})
		})
	})

	Describe("Cleanup", func() {
		It("removes the firewall rules applied to the container", func() {
			Expect(applier.Cleanup()).To(Succeed())

			Expect(netSh.RunHostCallCount()).To(Equal(2))

			expectedExistsArgs := []string{"advfirewall", "firewall", "show", "rule", `name="containerabc"`}
			Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedExistsArgs))

			expectedDeleteArgs := []string{"advfirewall", "firewall", "delete", "rule", `name="containerabc"`}
			Expect(netSh.RunHostArgsForCall(1)).To(Equal(expectedDeleteArgs))
		})

		Context("there are no firewall rules applied to the container", func() {
			BeforeEach(func() {
				netSh.RunHostReturnsOnCall(0, []byte{}, errors.New("firewall rule not found"))
			})

			It("does not error", func() {
				Expect(applier.Cleanup()).To(Succeed())

				Expect(netSh.RunHostCallCount()).To(Equal(1))

				expectedExistsArgs := []string{"advfirewall", "firewall", "show", "rule", `name="containerabc"`}
				Expect(netSh.RunHostArgsForCall(0)).To(Equal(expectedExistsArgs))
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
