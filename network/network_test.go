package network_test

import (
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/networkfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("NetworkManager", func() {
	const containerId = "some-container-id"

	var (
		networkManager  *network.NetworkManager
		netRuleApplier  *networkfakes.FakeNetRuleApplier
		hcsClient       *networkfakes.FakeHCSClient
		endpointManager *networkfakes.FakeEndpointManager
		config          network.Config
		hnsNetwork      *hcsshim.HNSNetwork
	)

	BeforeEach(func() {
		hcsClient = &networkfakes.FakeHCSClient{}
		netRuleApplier = &networkfakes.FakeNetRuleApplier{}
		endpointManager = &networkfakes.FakeEndpointManager{}
		config = network.Config{
			MTU:            1434,
			SubnetRange:    "123.45.0.0/67",
			GatewayAddress: "123.45.0.1",
			NetworkName:    "unit-test-name",
		}

		networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, endpointManager, containerId, config)

		logrus.SetOutput(ioutil.Discard)
	})

	Describe("CreateHostNATNetwork", func() {
		BeforeEach(func() {
			hcsClient.GetHNSNetworkByNameReturns(nil, errors.New("Network unit-test-name not found"))
		})

		It("creates the network with the correct values", func() {
			Expect(networkManager.CreateHostNATNetwork()).To(Succeed())

			Expect(hcsClient.GetHNSNetworkByNameCallCount()).To(Equal(1))
			Expect(hcsClient.GetHNSNetworkByNameArgsForCall(0)).To(Equal("unit-test-name"))

			Expect(hcsClient.CreateNetworkCallCount()).To(Equal(1))
			net := hcsClient.CreateNetworkArgsForCall(0)
			Expect(net.Name).To(Equal("unit-test-name"))
			Expect(net.Subnets).To(ConsistOf(hcsshim.Subnet{AddressPrefix: "123.45.0.0/67", GatewayAddress: "123.45.0.1"}))

			Expect(netRuleApplier.NatMTUCallCount()).To(Equal(1))
			Expect(netRuleApplier.NatMTUArgsForCall(0)).To(Equal(1434))
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
				netRuleApplier.NatMTUReturns(errors.New("couldn't set MTU on NAT network"))
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
				hcsClient.GetHNSNetworkByNameReturnsOnCall(0, nil, errors.New("Network unit-test-name not found"))
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
			localIP         string
			policy1         hcsshim.NatPolicy
			policy2         hcsshim.NatPolicy
		)

		BeforeEach(func() {
			var err error
			localIP, err = localip.LocalIP()
			Expect(err).NotTo(HaveOccurred())

			createdEndpoint = hcsshim.HNSEndpoint{}

			inputs = network.UpInputs{
				Pid: 1234,
				NetIn: []netrules.NetIn{
					{HostPort: 0, ContainerPort: 666},
					{HostPort: 0, ContainerPort: 888},
				},
				NetOut: []netrules.NetOut{
					{Protocol: 7},
					{Protocol: 8},
				},
			}

			policy1 = hcsshim.NatPolicy{ExternalPort: 111, InternalPort: 666}
			policy2 = hcsshim.NatPolicy{ExternalPort: 222, InternalPort: 888}

			netRuleApplier.InReturnsOnCall(0, policy1, nil)
			netRuleApplier.InReturnsOnCall(1, policy2, nil)

			endpointManager.CreateReturns(createdEndpoint, nil)
		})

		It("creates an endpoint with the port mappings, applies net out and mtu, and returns the up outputs", func() {
			output, err := networkManager.Up(inputs)
			Expect(err).NotTo(HaveOccurred())

			Expect(output.Properties.ContainerIP).To(Equal(localIP))
			Expect(output.Properties.DeprecatedHostIP).To(Equal("255.255.255.255"))
			Expect(output.Properties.MappedPorts).To(Equal(`[{"HostPort":111,"ContainerPort":666},{"HostPort":222,"ContainerPort":888}]`))

			Expect(endpointManager.CreateCallCount()).To(Equal(1))
			Expect(endpointManager.CreateArgsForCall(0)).To(Equal([]hcsshim.NatPolicy{policy1, policy2}))

			Expect(netRuleApplier.OutCallCount()).To(Equal(2))
			rule, ep := netRuleApplier.OutArgsForCall(0)
			Expect(rule).To(Equal(netrules.NetOut{Protocol: 7}))
			Expect(ep).To(Equal(createdEndpoint))

			rule, ep = netRuleApplier.OutArgsForCall(1)
			Expect(rule).To(Equal(netrules.NetOut{Protocol: 8}))
			Expect(ep).To(Equal(createdEndpoint))

			Expect(netRuleApplier.ContainerMTUCallCount()).To(Equal(1))
			mtu := netRuleApplier.ContainerMTUArgsForCall(0)
			Expect(mtu).To(Equal(1434))
		})

		Context("net in fails", func() {
			BeforeEach(func() {
				netRuleApplier.InReturnsOnCall(0, hcsshim.NatPolicy{}, errors.New("couldn't allocate port"))
			})

			It("cleans up allocated ports", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).To(MatchError("couldn't allocate port"))
				Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
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
				netRuleApplier.OutReturns(errors.New("couldn't set firewall rules"))
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
				netRuleApplier.ContainerMTUReturns(errors.New("couldn't set MTU"))
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
