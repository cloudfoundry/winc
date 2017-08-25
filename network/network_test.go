package network_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/hcs/hcsfakes"
	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/networkfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Network", func() {
	const (
		containerId = "containerid-1234"
		networkId   = "networkid-5678"
	)

	var (
		networkManager *network.Manager
		portAllocator  *networkfakes.FakePortAllocator
		netRuleApplier *networkfakes.FakeNetRuleApplier
		hcsClient      *networkfakes.FakeHCSClient
		config         network.Config
	)

	BeforeEach(func() {
		hcsClient = &networkfakes.FakeHCSClient{}
		portAllocator = &networkfakes.FakePortAllocator{}
		netRuleApplier = &networkfakes.FakeNetRuleApplier{}
		config = network.Config{
			MTU: 1434,
		}

		networkManager = network.NewManager(hcsClient, portAllocator, netRuleApplier, config, containerId)

		logrus.SetOutput(ioutil.Discard)
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

			Expect(hcsClient.GetHNSEndpointByNameArgsForCall(0)).To(Equal("containerid-1234"))

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
			eId, mtu := netRuleApplier.MTUArgsForCall(0)
			Expect(eId).To(Equal("ep-987"))
			Expect(mtu).To(Equal(1434))
		})
	})

	Describe("Down", func() {
		It("cleans up the container network", func() {
			Expect(networkManager.Down()).To(Succeed())
			Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
		})
	})

	Describe("AtachEndpointToConfig", func() {
		var (
			port1                int
			port2                int
			endpoint             *hcsshim.HNSEndpoint
			expectedPortMappings []hcsshim.NatPolicy
		)

		BeforeEach(func() {
			port1 = 42
			port2 = 53

			portAllocator.AllocatePortReturnsOnCall(0, port1, nil)
			portAllocator.AllocatePortReturnsOnCall(1, port2, nil)

			hcsClient.GetHNSNetworkByNameReturns(&hcsshim.HNSNetwork{Id: networkId, Name: "winc-nat"}, nil)

			endpoint = &hcsshim.HNSEndpoint{
				Id: "endpoint-id",
			}
			hcsClient.CreateEndpointReturns(endpoint, nil)

			expectedPortMappings = []hcsshim.NatPolicy{
				{Type: "NAT",
					Protocol:     "TCP",
					InternalPort: 2222,
					ExternalPort: 53},
				{Type: "NAT",
					Protocol:     "TCP",
					InternalPort: 8080,
					ExternalPort: 42},
			}
		})

		It("creates an endpoint on the nat network with two allocated ports", func() {
			config := hcsshim.ContainerConfig{}
			var err error
			config, err = networkManager.AttachEndpointToConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.EndpointList).To(Equal([]string{endpoint.Id}))

			Expect(portAllocator.AllocatePortCallCount()).To(Equal(2))
			handle, requestedPort := portAllocator.AllocatePortArgsForCall(0)
			Expect(handle).To(Equal(containerId))
			Expect(requestedPort).To(Equal(0))

			handle, requestedPort = portAllocator.AllocatePortArgsForCall(1)
			Expect(handle).To(Equal(containerId))
			Expect(requestedPort).To(Equal(0))

			Expect(hcsClient.GetHNSNetworkByNameCallCount()).To(Equal(1))

			Expect(hcsClient.CreateEndpointCallCount()).To(Equal(1))
			endpointToCreate := hcsClient.CreateEndpointArgsForCall(0)
			Expect(endpointToCreate.VirtualNetwork).To(Equal(networkId))
			Expect(endpointToCreate.Name).To(Equal(containerId))
			Expect(len(endpointToCreate.Policies)).To(Equal(2))

			requestedPortMappings := []hcsshim.NatPolicy{}
			for _, pol := range endpointToCreate.Policies {
				mapping := hcsshim.NatPolicy{}

				Expect(json.Unmarshal(pol, &mapping)).To(Succeed())
				Expect(mapping.Type).To(Equal(hcsshim.PolicyType("NAT")))
				requestedPortMappings = append(requestedPortMappings, mapping)
			}
			Expect(requestedPortMappings).To(ConsistOf(expectedPortMappings))
		})

		Context("winc-nat network does not already exist", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturns(nil, errors.New("Network winc-nat not found"))
				hcsClient.CreateNetworkReturns(&hcsshim.HNSNetwork{Id: networkId, Name: "winc-nat"}, nil)
			})

			It("creates winc-nat", func() {
				config := hcsshim.ContainerConfig{}
				var err error
				_, err = networkManager.AttachEndpointToConfig(config)
				Expect(err).NotTo(HaveOccurred())

				newNAT := hcsClient.CreateNetworkArgsForCall(0)
				Expect(newNAT.Name).To(Equal("winc-nat"))
				Expect(newNAT.Type).To(Equal("nat"))
				Expect(newNAT.Subnets).To(ConsistOf(hcsshim.Subnet{AddressPrefix: "172.35.0.0/22", GatewayAddress: "172.35.0.1"}))
			})

			Context("creating winc-nat fails", func() {
				BeforeEach(func() {
					hcsClient.CreateNetworkReturns(nil, errors.New("HNS failed with error : something happened"))
				})

				It("errors", func() {
					config := hcsshim.ContainerConfig{}
					var err error
					config, err = networkManager.AttachEndpointToConfig(config)
					Expect(err).To(HaveOccurred())
				})

				Context("because it already exists", func() {
					BeforeEach(func() {
						hcsClient.CreateNetworkReturns(nil, errors.New("HNS failed with error : {Object Exists}"))
						hcsClient.GetHNSNetworkByNameReturnsOnCall(2, &hcsshim.HNSNetwork{Id: networkId, Name: "winc-nat"}, nil)
					})

					It("retries until the network can be found", func() {
						config := hcsshim.ContainerConfig{}
						var err error
						config, err = networkManager.AttachEndpointToConfig(config)
						Expect(err).NotTo(HaveOccurred())

						Expect(config.EndpointList).To(Equal([]string{endpoint.Id}))
					})

					Context("when it hits the retry limit for finding the network", func() {
						BeforeEach(func() {
							// override the call 2 from the outer context
							hcsClient.GetHNSNetworkByNameReturnsOnCall(2, nil, errors.New("Network winc-nat not found"))
						})

						It("errors", func() {
							_, err := networkManager.AttachEndpointToConfig(hcsshim.ContainerConfig{})
							Expect(err).To(MatchError(&network.NoNATNetworkError{Name: "winc-nat"}))
						})
					})
				})
			})
		})

		Context("creating the endpoint fails", func() {
			Context("when the error is an unspecified HNS error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturnsOnCall(0, nil, errors.New("HNS failed with error : Unspecified error"))
					hcsClient.CreateEndpointReturnsOnCall(1, nil, errors.New("HNS failed with error : Unspecified error"))
					hcsClient.CreateEndpointReturnsOnCall(2, endpoint, nil)
				})

				It("retries creating the endpoint", func() {
					config := hcsshim.ContainerConfig{}
					var err error
					config, err = networkManager.AttachEndpointToConfig(config)
					Expect(err).NotTo(HaveOccurred())
					Expect(config.EndpointList).To(Equal([]string{endpoint.Id}))
				})
			})

			Context("it fails 3 times with an unspecified HNS error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("HNS failed with error : Unspecified error"))
				})

				It("returns an error and deallocates the ports", func() {
					_, err := networkManager.AttachEndpointToConfig(hcsshim.ContainerConfig{})
					Expect(err).To(MatchError("HNS failed with error : Unspecified error"))
					Expect(hcsClient.CreateEndpointCallCount()).To(Equal(3))

					Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
					Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))
				})
			})

			Context("when the error is some other error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("cannot create endpoint"))
				})

				It("deallocates the port", func() {
					_, err := networkManager.AttachEndpointToConfig(hcsshim.ContainerConfig{})
					Expect(err).To(MatchError("cannot create endpoint"))
					Expect(hcsClient.CreateEndpointCallCount()).To(Equal(1))

					Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
					Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))
				})
			})
		})
	})

	Describe("DeleteContainerEndpoints", func() {
		var (
			fakeContainer        *hcsfakes.FakeContainer
			endpoint1, endpoint2 *hcsshim.HNSEndpoint
		)

		BeforeEach(func() {
			fakeContainer = &hcsfakes.FakeContainer{}

			fakeContainer.StatisticsReturns(hcsshim.Statistics{
				Network: []hcsshim.NetworkStats{
					{EndpointId: "endpoint1"},
					{EndpointId: "endpoint2"},
				},
			}, nil)
			endpoint1 = &hcsshim.HNSEndpoint{Id: "endpoint1"}
			endpoint2 = &hcsshim.HNSEndpoint{Id: "endpoint2"}
			hcsClient.GetHNSEndpointByIDReturnsOnCall(0, endpoint1, nil)
			hcsClient.GetHNSEndpointByIDReturnsOnCall(1, endpoint2, nil)
		})

		It("deletes all endpoint and port mappings for the container", func() {
			Expect(networkManager.DeleteContainerEndpoints(fakeContainer)).To(Succeed())

			Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(2))
			Expect(hcsClient.DeleteEndpointArgsForCall(0)).To(Equal(endpoint1))
			Expect(hcsClient.DeleteEndpointArgsForCall(1)).To(Equal(endpoint2))

			Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
			Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))
		})

		Context("when deleting an endpoint fails", func() {
			BeforeEach(func() {
				hcsClient.DeleteEndpointReturnsOnCall(0, nil, errors.New("cannot delete endpoint 1"))
				hcsClient.DeleteEndpointReturnsOnCall(1, nil, nil)
			})

			It("continues to delete all other endpoints", func() {
				err := networkManager.DeleteContainerEndpoints(fakeContainer)
				Expect(err).To(MatchError("cannot delete endpoint 1"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(2))
				Expect(hcsClient.DeleteEndpointArgsForCall(1)).To(Equal(endpoint2))
			})

			It("does not release the ports", func() {
				err := networkManager.DeleteContainerEndpoints(fakeContainer)
				Expect(err).To(MatchError("cannot delete endpoint 1"))
				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(0))
			})
		})
	})

	Describe("DeleteEndpointsById", func() {
		var (
			endpoint1, endpoint2 *hcsshim.HNSEndpoint
		)

		BeforeEach(func() {
			endpoint1 = &hcsshim.HNSEndpoint{Id: "endpoint1"}
			endpoint2 = &hcsshim.HNSEndpoint{Id: "endpoint2"}
			hcsClient.GetHNSEndpointByIDReturnsOnCall(0, endpoint1, nil)
			hcsClient.GetHNSEndpointByIDReturnsOnCall(1, endpoint2, nil)
		})

		It("deletes all endpoints and port mappings for the container", func() {
			Expect(networkManager.DeleteEndpointsById([]string{endpoint1.Id, endpoint2.Id})).To(Succeed())

			Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(2))
			Expect(hcsClient.DeleteEndpointArgsForCall(0)).To(Equal(endpoint1))
			Expect(hcsClient.DeleteEndpointArgsForCall(1)).To(Equal(endpoint2))

			Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
			Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))
		})

		Context("when deleting an endpoint fails", func() {
			BeforeEach(func() {
				hcsClient.DeleteEndpointReturnsOnCall(0, nil, errors.New("cannot delete endpoint 1"))
				hcsClient.DeleteEndpointReturnsOnCall(1, nil, nil)
			})

			It("continues to delete all other endpoints", func() {
				err := networkManager.DeleteEndpointsById([]string{endpoint1.Id, endpoint2.Id})
				Expect(err).To(MatchError("cannot delete endpoint 1"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(2))
				Expect(hcsClient.DeleteEndpointArgsForCall(1)).To(Equal(endpoint2))
			})

			It("does not release the ports", func() {
				err := networkManager.DeleteEndpointsById([]string{endpoint1.Id, endpoint2.Id})
				Expect(err).To(MatchError("cannot delete endpoint 1"))
				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(0))
			})
		})
	})
})
