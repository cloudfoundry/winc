package network_test

import (
	"errors"
	"io/ioutil"
	"net"

	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/fakes"
	"code.cloudfoundry.org/winc/network/netrules"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("NetworkManager", func() {
	const containerId = "some-container-id"

	var (
		networkManager  *network.NetworkManager
		netRuleApplier  *fakes.NetRuleApplier
		hcsClient       *fakes.HCSClient
		endpointManager *fakes.EndpointManager
		mtu             *fakes.Mtu
		hnsNetwork      *hcsshim.HNSNetwork
		config          network.Config
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		netRuleApplier = &fakes.NetRuleApplier{}
		endpointManager = &fakes.EndpointManager{}
		mtu = &fakes.Mtu{}
		config = network.Config{
			MTU:            1434,
			SubnetRange:    "123.45.0.0/67",
			GatewayAddress: "123.45.0.1",
			NetworkName:    "unit-test-name",
		}

		networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, endpointManager, containerId, config, mtu)

		logrus.SetOutput(ioutil.Discard)
	})

	Describe("CreateHostNATNetwork", func() {
		BeforeEach(func() {
			hcsClient.GetHNSNetworkByNameReturns(nil, hcsshim.NetworkNotFoundError{NetworkName: "unit-test-name"})
		})

		It("creates the network with the correct values", func() {
			Expect(networkManager.CreateHostNATNetwork()).To(Succeed())

			Expect(hcsClient.GetHNSNetworkByNameCallCount()).To(Equal(1))
			Expect(hcsClient.GetHNSNetworkByNameArgsForCall(0)).To(Equal("unit-test-name"))

			Expect(hcsClient.CreateNetworkCallCount()).To(Equal(1))
			net, _ := hcsClient.CreateNetworkArgsForCall(0)
			Expect(net.Name).To(Equal("unit-test-name"))
			Expect(net.Subnets).To(ConsistOf(hcsshim.Subnet{AddressPrefix: "123.45.0.0/67", GatewayAddress: "123.45.0.1"}))
			Expect(net.DNSSuffix).To(Equal(""))

			Expect(mtu.SetNatCallCount()).To(Equal(1))
			Expect(mtu.SetNatArgsForCall(0)).To(Equal(1434))
		})

		Context("DNSSuffix is provided", func() {
			BeforeEach(func() {
				config.DNSSuffix = []string{"example1-dns-suffix", "example2-dns-suffix"}
				networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, endpointManager, containerId, config, mtu)
			})

			It("creates the network with the correct DNSSuffix values", func() {
				Expect(networkManager.CreateHostNATNetwork()).To(Succeed())
				net, _ := hcsClient.CreateNetworkArgsForCall(0)
				Expect(net.DNSSuffix).To(Equal("example1-dns-suffix,example2-dns-suffix"))
			})
		})

		Context("DNSSuffix value is invalid", func() {
			BeforeEach(func() {
				config.DNSSuffix = []string{"example1-dns-suffix", "example2,dns-suffix"}
				networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, endpointManager, containerId, config, mtu)
			})

			It("returns an error", func() {
				err := networkManager.CreateHostNATNetwork()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("Invalid DNSSuffix. First invalid DNSSuffix: example2,dns-suffix")))
			})
		})

		Context("the network already exists with the correct values", func() {
			BeforeEach(func() {
				hnsNetwork = &hcsshim.HNSNetwork{
					Name:    "unit-test-name",
					Subnets: []hcsshim.Subnet{{AddressPrefix: "123.45.0.0/67", GatewayAddress: "123.45.0.1"}},
				}
				hcsClient.GetHNSNetworkByNameReturns(hnsNetwork, nil)
			})

			It("does not create the network", func() {
				Expect(networkManager.CreateHostNATNetwork()).To(Succeed())

				Expect(hcsClient.GetHNSNetworkByNameCallCount()).To(Equal(1))
				Expect(hcsClient.GetHNSNetworkByNameArgsForCall(0)).To(Equal("unit-test-name"))
				Expect(hcsClient.CreateNetworkCallCount()).To(Equal(0))
			})
		})

		Context("the network already exists with an incorrect address prefix", func() {
			BeforeEach(func() {
				hnsNetwork = &hcsshim.HNSNetwork{
					Name:    "unit-test-name",
					Subnets: []hcsshim.Subnet{{AddressPrefix: "123.89.0.0/67", GatewayAddress: "123.45.0.1"}},
				}
				hcsClient.GetHNSNetworkByNameReturns(hnsNetwork, nil)
			})

			It("returns an error", func() {
				err := networkManager.CreateHostNATNetwork()
				Expect(err).To(BeAssignableToTypeOf(&network.SameNATNetworkNameError{}))
			})
		})

		Context("the network already exists with an incorrect gateway address", func() {
			BeforeEach(func() {
				hnsNetwork = &hcsshim.HNSNetwork{
					Name:    "unit-test-name",
					Subnets: []hcsshim.Subnet{{AddressPrefix: "123.45.0.0/67", GatewayAddress: "123.45.67.89"}},
				}
				hcsClient.GetHNSNetworkByNameReturns(hnsNetwork, nil)
			})

			It("returns an error", func() {
				err := networkManager.CreateHostNATNetwork()
				Expect(err).To(BeAssignableToTypeOf(&network.SameNATNetworkNameError{}))
			})
		})

		Context("GetHNSNetwork returns a non network not found error", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturns(nil, errors.New("some HNS error"))
			})

			It("returns an error", func() {
				err := networkManager.CreateHostNATNetwork()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("CreateNetwork returns an error", func() {
			BeforeEach(func() {
				hcsClient.CreateNetworkReturns(nil, errors.New("couldn't create HNS network"))
			})

			It("returns an error", func() {
				err := networkManager.CreateHostNATNetwork()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("NatMTU returns an error", func() {
			BeforeEach(func() {
				mtu.SetNatReturns(errors.New("couldn't set MTU on NAT network"))
			})

			It("returns an error", func() {
				err := networkManager.CreateHostNATNetwork()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("DeleteHostNATNetwork", func() {
		BeforeEach(func() {
			hnsNetwork = &hcsshim.HNSNetwork{Name: "unit-test-name"}
			hcsClient.GetHNSNetworkByNameReturnsOnCall(0, hnsNetwork, nil)
		})

		It("deletes the network", func() {
			Expect(networkManager.DeleteHostNATNetwork()).To(Succeed())

			Expect(hcsClient.GetHNSNetworkByNameCallCount()).To(Equal(1))
			Expect(hcsClient.GetHNSNetworkByNameArgsForCall(0)).To(Equal("unit-test-name"))

			Expect(hcsClient.DeleteNetworkCallCount()).To(Equal(1))
			Expect(hcsClient.DeleteNetworkArgsForCall(0)).To(Equal(hnsNetwork))
		})

		Context("the network does not exist", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturnsOnCall(0, nil, hcsshim.NetworkNotFoundError{NetworkName: "unit-test-name"})
			})

			It("returns success", func() {
				Expect(networkManager.DeleteHostNATNetwork()).To(Succeed())

				Expect(hcsClient.GetHNSNetworkByNameCallCount()).To(Equal(1))
				Expect(hcsClient.GetHNSNetworkByNameArgsForCall(0)).To(Equal("unit-test-name"))
			})
		})

		Context("GetHNSNetwork returns a non network not found error", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturns(nil, errors.New("some HNS error"))
			})

			It("returns an error", func() {
				err := networkManager.CreateHostNATNetwork()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Up", func() {
		var (
			inputs          network.UpInputs
			createdEndpoint hcsshim.HNSEndpoint
			containerIP     net.IP
			nat1            *hcsshim.NatPolicy
			nat2            *hcsshim.NatPolicy
			inAcl1          *hcsshim.ACLPolicy
			inAcl2          *hcsshim.ACLPolicy
			outAcl1         *hcsshim.ACLPolicy
			outAcl2         *hcsshim.ACLPolicy
		)

		BeforeEach(func() {
			containerIP = net.ParseIP("111.222.33.44")

			createdEndpoint = hcsshim.HNSEndpoint{
				IPAddress: containerIP,
			}

			inputs = network.UpInputs{
				Pid:        1234,
				Properties: map[string]interface{}{"ports": "997,998,999"},
				NetIn: []netrules.NetIn{
					{HostPort: 0, ContainerPort: 666},
					{HostPort: 0, ContainerPort: 888},
				},
				NetOut: []netrules.NetOut{
					{Protocol: 6},
					{Protocol: 17},
				},
			}

			nat1 = &hcsshim.NatPolicy{
				Type:         hcsshim.Nat,
				Protocol:     "TCP",
				ExternalPort: 111,
				InternalPort: 666,
			}

			nat2 = &hcsshim.NatPolicy{
				Type:         hcsshim.Nat,
				Protocol:     "TCP",
				ExternalPort: 222,
				InternalPort: 888,
			}

			inAcl1 = &hcsshim.ACLPolicy{
				Type:       hcsshim.ACL,
				LocalPorts: "666",
				Direction:  hcsshim.In,
				Action:     hcsshim.Allow,
			}

			inAcl2 = &hcsshim.ACLPolicy{
				Type:       hcsshim.ACL,
				LocalPorts: "888",
				Direction:  hcsshim.In,
				Action:     hcsshim.Allow,
			}

			outAcl1 = &hcsshim.ACLPolicy{
				Type:      hcsshim.ACL,
				Direction: hcsshim.Out,
				Action:    hcsshim.Allow,
				Protocol:  6,
			}

			outAcl2 = &hcsshim.ACLPolicy{
				Type:      hcsshim.ACL,
				Direction: hcsshim.In,
				Action:    hcsshim.Allow,
				Protocol:  17,
			}

			netRuleApplier.InReturnsOnCall(0, nat1, inAcl1, nil)
			netRuleApplier.InReturnsOnCall(1, nat2, inAcl2, nil)

			netRuleApplier.OutReturnsOnCall(0, outAcl1, nil)
			netRuleApplier.OutReturnsOnCall(1, outAcl2, nil)

			endpointManager.CreateReturns(createdEndpoint, nil)
		})

		It("creates an endpoint, applies ports, applies net out, handles mtu, and returns the up outputs", func() {
			output, err := networkManager.Up(inputs)
			Expect(err).NotTo(HaveOccurred())

			Expect(output.Properties.ContainerIP).To(Equal(containerIP.String()))
			Expect(output.Properties.DeprecatedHostIP).To(Equal("255.255.255.255"))
			Expect(output.Properties.MappedPorts).To(Equal(`[{"HostPort":111,"ContainerPort":666},{"HostPort":222,"ContainerPort":888}]`))

			Expect(endpointManager.CreateCallCount()).To(Equal(1))

			Expect(netRuleApplier.InCallCount()).To(Equal(2))
			inRule, ip := netRuleApplier.InArgsForCall(0)
			Expect(inRule).To(Equal(netrules.NetIn{HostPort: 0, ContainerPort: 666}))
			Expect(ip).To(Equal(containerIP.String()))

			inRule, ip = netRuleApplier.InArgsForCall(1)
			Expect(inRule).To(Equal(netrules.NetIn{HostPort: 0, ContainerPort: 888}))
			Expect(ip).To(Equal(containerIP.String()))

			Expect(netRuleApplier.OpenPortCallCount()).To(Equal(3))
			portOne := netRuleApplier.OpenPortArgsForCall(0)
			Expect(portOne).To(Equal(uint32(997)))

			portTwo := netRuleApplier.OpenPortArgsForCall(1)
			Expect(portTwo).To(Equal(uint32(998)))

			portThree := netRuleApplier.OpenPortArgsForCall(2)
			Expect(portThree).To(Equal(uint32(999)))

			Expect(netRuleApplier.OutCallCount()).To(Equal(2))
			outRule, ip := netRuleApplier.OutArgsForCall(0)
			Expect(outRule).To(Equal(netrules.NetOut{Protocol: 6}))
			Expect(ip).To(Equal(containerIP.String()))

			outRule, ip = netRuleApplier.OutArgsForCall(1)
			Expect(outRule).To(Equal(netrules.NetOut{Protocol: 17}))
			Expect(ip).To(Equal(containerIP.String()))

			Expect(endpointManager.ApplyPoliciesCallCount()).To(Equal(1))
			ep, nats, acls := endpointManager.ApplyPoliciesArgsForCall(0)
			Expect(ep).To(Equal(createdEndpoint))

			expectedNatPolicies := []hcsshim.NatPolicy{*nat1, *nat2}
			var receivedNatPolicies []hcsshim.NatPolicy
			for _, v := range nats {
				receivedNatPolicies = append(receivedNatPolicies, *v)
			}
			Expect(receivedNatPolicies).To(Equal(expectedNatPolicies))

			expectedAclPolicies := []hcsshim.ACLPolicy{*inAcl1, *inAcl2, *outAcl1, *outAcl2}
			var receivedAclPolicies []hcsshim.ACLPolicy
			for _, v := range acls {
				receivedAclPolicies = append(receivedAclPolicies, *v)
			}
			Expect(receivedAclPolicies).To(Equal(expectedAclPolicies))

			Expect(mtu.SetContainerCallCount()).To(Equal(1))
			receivedMtu := mtu.SetContainerArgsForCall(0)
			Expect(receivedMtu).To(Equal(1434))
		})

		Context("when the config specifies DNS servers", func() {
			BeforeEach(func() {
				config := network.Config{
					DNSServers: []string{"1.1.1.1", "2.2.2.2"},
				}
				networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, endpointManager, containerId, config, mtu)
				inputs.NetOut = []netrules.NetOut{}
			})

			It("creates netout rules for the servers", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).NotTo(HaveOccurred())

				dnsServer1 := net.ParseIP("1.1.1.1")
				dnsServer2 := net.ParseIP("2.2.2.2")
				Expect(netRuleApplier.OutCallCount()).To(Equal(4))

				outRule, ip := netRuleApplier.OutArgsForCall(0)
				Expect(outRule).To(Equal(netrules.NetOut{
					Protocol: netrules.ProtocolTCP,
					Networks: []netrules.IPRange{{Start: dnsServer1, End: dnsServer1}},
					Ports:    []netrules.PortRange{{Start: 53, End: 53}},
				}))
				Expect(ip).To(Equal(containerIP.String()))

				outRule, ip = netRuleApplier.OutArgsForCall(1)
				Expect(outRule).To(Equal(netrules.NetOut{
					Protocol: netrules.ProtocolUDP,
					Networks: []netrules.IPRange{{Start: dnsServer1, End: dnsServer1}},
					Ports:    []netrules.PortRange{{Start: 53, End: 53}},
				}))
				Expect(ip).To(Equal(containerIP.String()))

				outRule, ip = netRuleApplier.OutArgsForCall(2)
				Expect(outRule).To(Equal(netrules.NetOut{
					Protocol: netrules.ProtocolTCP,
					Networks: []netrules.IPRange{{Start: dnsServer2, End: dnsServer2}},
					Ports:    []netrules.PortRange{{Start: 53, End: 53}},
				}))
				Expect(ip).To(Equal(containerIP.String()))

				outRule, ip = netRuleApplier.OutArgsForCall(3)
				Expect(outRule).To(Equal(netrules.NetOut{
					Protocol: netrules.ProtocolUDP,
					Networks: []netrules.IPRange{{Start: dnsServer2, End: dnsServer2}},
					Ports:    []netrules.PortRange{{Start: 53, End: 53}},
				}))
				Expect(ip).To(Equal(containerIP.String()))
			})
		})

		Context("when 'default_allow_outbound_traffic' flag is set AND inputs are not empty", func() {
			BeforeEach(func() {
				config := network.Config{AllowOutboundTrafficByDefault: true}
				networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, endpointManager, containerId, config, mtu)
				inputs = network.UpInputs{
					Pid:        1234,
					Properties: map[string]interface{}{},
					NetOut: []netrules.NetOut{
						{Protocol: 6},
						{Protocol: 17},
					}}
			})

			It("ignores the flag and preserves the specified input rules", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).NotTo(HaveOccurred())

				Expect(netRuleApplier.OutCallCount()).To(Equal(2))
				outRule, ip := netRuleApplier.OutArgsForCall(0)
				Expect(outRule).To(Equal(netrules.NetOut{Protocol: 6}))
				Expect(ip).To(Equal(containerIP.String()))

				outRule, ip = netRuleApplier.OutArgsForCall(1)
				Expect(outRule).To(Equal(netrules.NetOut{Protocol: 17}))
				Expect(ip).To(Equal(containerIP.String()))
			})
		})

		Context("when 'default_allow_outbound_traffic' flag not set AND inputs are empty", func() {
			BeforeEach(func() {
				config := network.Config{}
				networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, endpointManager, containerId, config, mtu)
				inputs = network.UpInputs{Pid: 1234, Properties: map[string]interface{}{}}
			})

			It("does not create outbound traffic netout rules", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).NotTo(HaveOccurred())

				Expect(netRuleApplier.OutCallCount()).To(BeZero())
			})
		})

		Context("when 'default_allow_outbound_traffic' flag is set AND inputs are empty", func() {
			BeforeEach(func() {
				config := network.Config{AllowOutboundTrafficByDefault: true}
				networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, endpointManager, containerId, config, mtu)
				inputs = network.UpInputs{Pid: 1234, Properties: map[string]interface{}{}}
			})

			It("creates allow all outbound traffic netout rules", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).NotTo(HaveOccurred())

				Expect(netRuleApplier.OutCallCount()).To(Equal(1))

				outRule, _ := netRuleApplier.OutArgsForCall(0)
				Expect(outRule).To(Equal(netrules.NetOut{
					Protocol: netrules.ProtocolAll,
				}))
			})
		})

		Context("net in fails", func() {
			BeforeEach(func() {
				netRuleApplier.InReturnsOnCall(0, nil, nil, errors.New("couldn't allocate port"))
			})

			It("cleans up allocated ports", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).To(MatchError("couldn't allocate port"))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
			})
		})

		Context("when input.Properties.ports does not exist", func() {
			BeforeEach(func() {
				inputs.Properties = map[string]interface{}{}
			})

			It("network up still runs successfully", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the port value is invalid", func() {
			BeforeEach(func() {
				inputs.Properties = map[string]interface{}{"ports": "banana"}
			})

			It("returns a helpful error message", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).To(MatchError(ContainSubstring("Invalid port in input.Properties.ports: banana, error")))
			})
		})

		Context("when applier failes to open the port", func() {
			BeforeEach(func() {
				netRuleApplier.OpenPortReturnsOnCall(0, errors.New("banana"))
			})

			It("cleans up allocated ports", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).To(MatchError(MatchRegexp("Failed to open port: [0-9]*, error: banana")))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
			})
		})

		Context("when the ports property is invalid", func() {
			BeforeEach(func() {
				inputs.Properties = map[string]interface{}{"ports": 999}
			})

			It("returns a helpful error message", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).To(MatchError("Invalid type input.Properties.ports: 999"))
			})
		})

		Context("endpoint create fails", func() {
			BeforeEach(func() {
				endpointManager.CreateReturns(hcsshim.HNSEndpoint{}, errors.New("couldn't create endpoint"))
			})

			It("cleans up allocated ports", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).To(MatchError("couldn't create endpoint"))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
			})
		})

		Context("net out fails", func() {
			BeforeEach(func() {
				netRuleApplier.OutReturnsOnCall(0, nil, errors.New("couldn't set firewall rules"))
			})

			It("cleans up allocated ports, firewall rules and deletes the endpoint", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).To(MatchError("couldn't set firewall rules"))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
				Expect(endpointManager.DeleteCallCount()).To(Equal(1))
			})
		})

		Context("MTU fails", func() {
			BeforeEach(func() {
				mtu.SetContainerReturns(errors.New("couldn't set MTU"))
			})

			It("cleans up allocated ports, firewall rules and deletes the endpoint", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).To(MatchError("couldn't set MTU"))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
				Expect(endpointManager.DeleteCallCount()).To(Equal(1))
			})
		})
	})

	Describe("Down", func() {
		It("deletes the endpoint and cleans up the ports and firewall rules", func() {
			Expect(networkManager.Down()).To(Succeed())
			Expect(endpointManager.DeleteCallCount()).To(Equal(1))
			Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
		})

		Context("endpoint delete fails", func() {
			BeforeEach(func() {
				endpointManager.DeleteReturns(errors.New("couldn't delete endpoint"))
			})

			It("cleans up allocated ports, firewall rules but returns an error", func() {
				Expect(networkManager.Down()).To(MatchError("couldn't delete endpoint"))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
				Expect(endpointManager.DeleteCallCount()).To(Equal(1))
			})
		})

		Context("host cleanup fails", func() {
			BeforeEach(func() {
				netRuleApplier.CleanupReturns(errors.New("couldn't remove firewall rules"))
			})

			It("deletes the endpoint but returns an error", func() {
				Expect(networkManager.Down()).To(MatchError("couldn't remove firewall rules"))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
				Expect(endpointManager.DeleteCallCount()).To(Equal(1))
			})
		})

		Context("host cleanup + endpoint delete fail", func() {
			BeforeEach(func() {
				endpointManager.DeleteReturns(errors.New("couldn't delete endpoint"))
				netRuleApplier.CleanupReturns(errors.New("couldn't remove firewall rules"))
			})

			It("deletes the endpoint but returns an error", func() {
				Expect(networkManager.Down()).To(MatchError("couldn't delete endpoint, couldn't remove firewall rules"))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
				Expect(endpointManager.DeleteCallCount()).To(Equal(1))
			})
		})
	})
})
