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
		networkManager *network.NetworkManager
		netRuleApplier *networkfakes.FakeNetRuleApplier
		hcsClient      *networkfakes.FakeHCSClient
		config         network.Config
		hnsNetwork     *hcsshim.HNSNetwork
	)

	BeforeEach(func() {
		hcsClient = &networkfakes.FakeHCSClient{}
		netRuleApplier = &networkfakes.FakeNetRuleApplier{}
		config = network.Config{
			MTU:            1434,
			SubnetRange:    "123.45.0.0/67",
			GatewayAddress: "123.45.0.1",
			NetworkName:    "unit-test-name",
		}

		networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, containerId, config)

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
			inputs   network.UpInputs
			endpoint *hcsshim.HNSEndpoint
			localIp  string
		)

		BeforeEach(func() {
			endpoint = &hcsshim.HNSEndpoint{
				Id: "ep-987",
			}
			hcsClient.GetHNSEndpointByNameReturns(endpoint, nil)

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

			netRuleApplier.InReturnsOnCall(0, netrules.PortMapping{HostPort: 555, ContainerPort: 666}, nil)
			netRuleApplier.InReturnsOnCall(1, netrules.PortMapping{HostPort: 777, ContainerPort: 888}, nil)

			var err error
			localIp, err = localip.LocalIP()
			Expect(err).ToNot(HaveOccurred())
		})

		It("applies NetIn rules, NetOut rules, and MTU", func() {
			outputs, err := networkManager.Up(inputs)
			Expect(err).ToNot(HaveOccurred())
			expectedUpOutputs := network.UpOutputs{}
			expectedUpOutputs.Properties.ContainerIP = localIp
			expectedUpOutputs.Properties.DeprecatedHostIP = "255.255.255.255"
			expectedUpOutputs.Properties.MappedPorts = `[{"HostPort":555,"ContainerPort":666},{"HostPort":777,"ContainerPort":888}]`
			expectedUpOutputs.DNSServers = nil
			Expect(outputs).To(Equal(expectedUpOutputs))

			Expect(hcsClient.GetHNSEndpointByNameArgsForCall(0)).To(Equal(containerId))

			Expect(netRuleApplier.InCallCount()).To(Equal(2))
			niRule, ep := netRuleApplier.InArgsForCall(0)
			Expect(niRule).To(Equal(netrules.NetIn{
				HostPort:      0,
				ContainerPort: 666,
			}))
			Expect(ep).To(Equal(endpoint))
			niRule, ep = netRuleApplier.InArgsForCall(1)
			Expect(niRule).To(Equal(netrules.NetIn{
				HostPort:      0,
				ContainerPort: 888,
			}))
			Expect(ep).To(Equal(endpoint))

			Expect(netRuleApplier.OutCallCount()).To(Equal(2))
			noRule, ep := netRuleApplier.OutArgsForCall(0)
			Expect(noRule).To(Equal(netrules.NetOut{
				Protocol: 7,
			}))
			Expect(ep).To(Equal(endpoint))
			noRule, ep = netRuleApplier.OutArgsForCall(1)
			Expect(noRule).To(Equal(netrules.NetOut{
				Protocol: 8,
			}))
			Expect(ep).To(Equal(endpoint))

			Expect(netRuleApplier.MTUCallCount()).To(Equal(1))
			actualContainerId, actualMtu := netRuleApplier.MTUArgsForCall(0)
			Expect(actualContainerId).To(Equal(containerId))
			Expect(actualMtu).To(Equal(1434))
		})
	})

	Describe("Down", func() {
		It("cleans up the container network", func() {
			Expect(networkManager.Down()).To(Succeed())
			Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
		})
	})
})
