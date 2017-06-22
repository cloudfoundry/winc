package network_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/winc/hcsclient/hcsclientfakes"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/networkfakes"
	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			port                int
			networkId           string
			containerId         string
			endpoint            *hcsshim.HNSEndpoint
			expectedPortMapping hcsshim.NatPolicy
		)

		BeforeEach(func() {
			port = 42
			portAllocator.AllocatePortReturns(port, nil)

			networkId = "network-id"
			hcsClient.GetHNSNetworkByNameReturns(&hcsshim.HNSNetwork{Id: networkId}, nil)

			containerId = "container-id"

			endpoint = &hcsshim.HNSEndpoint{
				Id: "endpoint-id",
			}
			hcsClient.CreateEndpointReturns(endpoint, nil)

			expectedPortMapping = hcsshim.NatPolicy{
				Type:         "NAT",
				Protocol:     "TCP",
				InternalPort: 8080,
				ExternalPort: 42,
			}
		})

		It("creates an endpoint on the nat network using an allocated port", func() {
			config := hcsshim.ContainerConfig{}
			var err error
			config, err = networkManager.AttachEndpointToConfig(config, containerId)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.EndpointList).To(Equal([]string{endpoint.Id}))

			Expect(portAllocator.AllocatePortCallCount()).To(Equal(1))
			handle, requestedPort := portAllocator.AllocatePortArgsForCall(0)
			Expect(handle).To(Equal(containerId))
			Expect(requestedPort).To(Equal(0))

			Expect(hcsClient.GetHNSNetworkByNameCallCount()).To(Equal(1))
			Expect(hcsClient.GetHNSNetworkByNameArgsForCall(0)).To(Equal("nat"))

			Expect(hcsClient.CreateEndpointCallCount()).To(Equal(1))
			endpointToCreate := hcsClient.CreateEndpointArgsForCall(0)
			Expect(endpointToCreate.VirtualNetwork).To(Equal(networkId))
			Expect(endpointToCreate.Name).To(Equal(containerId))
			Expect(len(endpointToCreate.Policies)).To(Equal(1))

			var requestedPortMapping hcsshim.NatPolicy
			Expect(json.Unmarshal(endpointToCreate.Policies[0], &requestedPortMapping)).To(Succeed())
			Expect(requestedPortMapping).To(Equal(expectedPortMapping))
		})

		Context("when getting the network fails", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturns(nil, errors.New("cannot get network"))
			})

			It("deallocates the port", func() {
				_, err := networkManager.AttachEndpointToConfig(hcsshim.ContainerConfig{}, containerId)
				Expect(err).To(MatchError("cannot get network"))

				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
				Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))
			})
		})

		Context("creating the endpoint fails", func() {
			BeforeEach(func() {
				hcsClient.CreateEndpointReturns(nil, errors.New("cannot create endpoint"))
			})

			It("deallocates the port", func() {
				_, err := networkManager.AttachEndpointToConfig(hcsshim.ContainerConfig{}, containerId)
				Expect(err).To(MatchError("cannot create endpoint"))

				Expect(portAllocator.ReleaseAllPortsCallCount()).To(Equal(1))
				Expect(portAllocator.ReleaseAllPortsArgsForCall(0)).To(Equal(containerId))
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
