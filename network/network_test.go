package network_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/networkfakes"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Network", func() {
	var (
		networkManager network.NetworkManager
		portAllocator  *networkfakes.FakePortAllocator
		hcsClient      *hcsclientfakes.FakeClient
	)

	BeforeEach(func() {
		hcsClient = &hcsclientfakes.FakeClient{}
		portAllocator = &networkfakes.FakePortAllocator{}
		networkManager = network.NewNetworkManager(hcsClient, portAllocator)

		logrus.SetOutput(ioutil.Discard)
	})

	Describe("AtachEndpointToConfig", func() {
		var (
			port1                int
			port2                int
			networkId            string
			containerId          string
			endpoint             *hcsshim.HNSEndpoint
			expectedPortMappings []hcsshim.NatPolicy
		)

		BeforeEach(func() {
			port1 = 42
			port2 = 53

			portAllocator.AllocatePortReturnsOnCall(0, port1, nil)
			portAllocator.AllocatePortReturnsOnCall(1, port2, nil)

			networkId = "network-id"
			hcsClient.HNSListNetworkRequestReturns([]hcsshim.HNSNetwork{{Id: networkId, Name: network.WINC_NETWORK}}, nil)

			containerId = "container-id"

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
			config, err = networkManager.AttachEndpointToConfig(config, containerId)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.EndpointList).To(Equal([]string{endpoint.Id}))

			Expect(portAllocator.AllocatePortCallCount()).To(Equal(2))
			handle, requestedPort := portAllocator.AllocatePortArgsForCall(0)
			Expect(handle).To(Equal(containerId))
			Expect(requestedPort).To(Equal(0))

			handle, requestedPort = portAllocator.AllocatePortArgsForCall(1)
			Expect(handle).To(Equal(containerId))
			Expect(requestedPort).To(Equal(0))

			Expect(hcsClient.HNSListNetworkRequestCallCount()).To(Equal(1))

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
			var oldNAT hcsshim.HNSNetwork

			BeforeEach(func() {
				oldNAT = hcsshim.HNSNetwork{Id: networkId, Name: "some-other-nat-network", Type: "nat"}
				hcsClient.HNSListNetworkRequestReturns([]hcsshim.HNSNetwork{oldNAT}, nil)
				hcsClient.CreateNetworkReturns(&hcsshim.HNSNetwork{Id: networkId, Name: network.WINC_NETWORK}, nil)
			})

			It("deletes any existing nat networks and creates winc-nat", func() {
				config := hcsshim.ContainerConfig{}
				var err error
				_, err = networkManager.AttachEndpointToConfig(config, containerId)
				Expect(err).NotTo(HaveOccurred())

				Expect(hcsClient.DeleteNetworkArgsForCall(0)).To(Equal(&oldNAT))
				newNAT := hcsClient.CreateNetworkArgsForCall(0)
				Expect(newNAT.Name).To(Equal("winc-nat"))
				Expect(newNAT.Type).To(Equal("nat"))
				Expect(newNAT.Subnets).To(ConsistOf(hcsshim.Subnet{AddressPrefix: "172.35.0.0/22", GatewayAddress: "172.35.0.1"}))
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
					config, err = networkManager.AttachEndpointToConfig(config, containerId)
					Expect(err).NotTo(HaveOccurred())
					Expect(config.EndpointList).To(Equal([]string{endpoint.Id}))
				})
			})

			Context("it fails 3 times with an unspecified HNS error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("HNS failed with error : Unspecified error"))
				})

				It("returns an error and deallocates the ports", func() {
					_, err := networkManager.AttachEndpointToConfig(hcsshim.ContainerConfig{}, containerId)
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
					_, err := networkManager.AttachEndpointToConfig(hcsshim.ContainerConfig{}, containerId)
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
			fakeContainer        *hcsclientfakes.FakeContainer
			containerId          string
			endpoint1, endpoint2 *hcsshim.HNSEndpoint
		)

		BeforeEach(func() {
			fakeContainer = &hcsclientfakes.FakeContainer{}
			containerId = "container-id"

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
			Expect(networkManager.DeleteContainerEndpoints(fakeContainer, containerId)).To(Succeed())

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
				err := networkManager.DeleteContainerEndpoints(fakeContainer, containerId)
				Expect(err).To(MatchError("cannot delete endpoint 1"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(2))
				Expect(hcsClient.DeleteEndpointArgsForCall(1)).To(Equal(endpoint2))
			})

			It("does not release the ports", func() {
				err := networkManager.DeleteContainerEndpoints(fakeContainer, containerId)
				Expect(err).To(MatchError("cannot delete endpoint 1"))
				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(0))
			})
		})
	})

	Describe("DeleteEndpointsById", func() {
		var (
			containerId          string
			endpoint1, endpoint2 *hcsshim.HNSEndpoint
		)

		BeforeEach(func() {
			containerId = "container-id"

			endpoint1 = &hcsshim.HNSEndpoint{Id: "endpoint1"}
			endpoint2 = &hcsshim.HNSEndpoint{Id: "endpoint2"}
			hcsClient.GetHNSEndpointByIDReturnsOnCall(0, endpoint1, nil)
			hcsClient.GetHNSEndpointByIDReturnsOnCall(1, endpoint2, nil)
		})

		It("deletes all endpoints and port mappings for the container", func() {
			Expect(networkManager.DeleteEndpointsById([]string{endpoint1.Id, endpoint2.Id}, containerId)).To(Succeed())

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
				err := networkManager.DeleteEndpointsById([]string{endpoint1.Id, endpoint2.Id}, containerId)
				Expect(err).To(MatchError("cannot delete endpoint 1"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(2))
				Expect(hcsClient.DeleteEndpointArgsForCall(1)).To(Equal(endpoint2))
			})

			It("does not release the ports", func() {
				err := networkManager.DeleteEndpointsById([]string{endpoint1.Id, endpoint2.Id}, containerId)
				Expect(err).To(MatchError("cannot delete endpoint 1"))
				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(0))
			})
		})
	})
})
