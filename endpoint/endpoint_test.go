package endpoint_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"code.cloudfoundry.org/winc/endpoint"
	"code.cloudfoundry.org/winc/endpoint/endpointfakes"
	"code.cloudfoundry.org/winc/network"
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("EndpointManager", func() {
	const (
		containerId = "containerid-1234"
		networkId   = "networkid-5678"
		endpointId  = "endpointid-abcd"
		networkName = "some-network-name"
	)

	var (
		endpointManager *endpoint.EndpointManager
		hcsClient       *endpointfakes.FakeHCSClient
	)

	BeforeEach(func() {
		hcsClient = &endpointfakes.FakeHCSClient{}
		config := network.Config{
			NetworkName: networkName,
			DNSServers:  []string{"1.1.1.1", "2.2.2.2"},
		}

		endpointManager = endpoint.NewEndpointManager(hcsClient, containerId, config)

		logrus.SetOutput(ioutil.Discard)
	})

	Describe("Create", func() {
		var (
			policy1 hcsshim.NatPolicy
			policy2 hcsshim.NatPolicy
		)

		BeforeEach(func() {
			policy1 = hcsshim.NatPolicy{InternalPort: 111, ExternalPort: 222}
			policy2 = hcsshim.NatPolicy{InternalPort: 333, ExternalPort: 444}

			hcsClient.GetHNSNetworkByNameReturns(&hcsshim.HNSNetwork{Id: networkId, Name: networkName}, nil)
			hcsClient.CreateEndpointReturns(&hcsshim.HNSEndpoint{Id: endpointId}, nil)
		})

		It("creates an endpoint on the configured network and attaches it to the container", func() {
			ep, err := endpointManager.Create([]hcsshim.NatPolicy{policy1, policy2})
			Expect(err).NotTo(HaveOccurred())
			Expect(ep.Id).To(Equal(endpointId))

			Expect(hcsClient.CreateEndpointCallCount()).To(Equal(1))
			endpointToCreate := hcsClient.CreateEndpointArgsForCall(0)
			Expect(endpointToCreate.VirtualNetwork).To(Equal(networkId))
			Expect(endpointToCreate.Name).To(Equal(containerId))
			Expect(len(endpointToCreate.Policies)).To(Equal(2))
			Expect(endpointToCreate.DNSServerList).To(Equal("1.1.1.1,2.2.2.2"))

			requestedPortMappings := []hcsshim.NatPolicy{}
			for _, pol := range endpointToCreate.Policies {
				mapping := hcsshim.NatPolicy{}

				Expect(json.Unmarshal(pol, &mapping)).To(Succeed())
				requestedPortMappings = append(requestedPortMappings, mapping)
			}
			Expect(requestedPortMappings).To(ConsistOf([]hcsshim.NatPolicy{policy1, policy2}))

			Expect(hcsClient.HotAttachEndpointCallCount()).To(Equal(1))
			cId, eId := hcsClient.HotAttachEndpointArgsForCall(0)
			Expect(cId).To(Equal(containerId))
			Expect(eId).To(Equal(endpointId))
		})

		Context("the network does not already exist", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturns(nil, fmt.Errorf("Network %s not found", networkName))
			})

			It("returns an error", func() {
				_, err := endpointManager.Create(nil)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("creating the endpoint fails", func() {
			Context("when the error is an unspecified HNS error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturnsOnCall(0, nil, errors.New("HNS failed with error : Unspecified error"))
					hcsClient.CreateEndpointReturnsOnCall(1, nil, errors.New("HNS failed with error : Unspecified error"))
					hcsClient.CreateEndpointReturnsOnCall(2, &hcsshim.HNSEndpoint{Id: endpointId}, nil)
				})

				It("retries creating the endpoint", func() {
					ep, err := endpointManager.Create(nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(ep.Id).To(Equal(endpointId))
				})
			})

			Context("it fails 3 times with an unspecified HNS error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("HNS failed with error : Unspecified error"))
				})

				It("returns an error", func() {
					_, err := endpointManager.Create(nil)
					Expect(err).To(MatchError("HNS failed with error : Unspecified error"))
					Expect(hcsClient.CreateEndpointCallCount()).To(Equal(3))
				})
			})

			Context("when the error is some other error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("cannot create endpoint"))
				})

				It("does not retry", func() {
					_, err := endpointManager.Create(nil)
					Expect(err).To(MatchError("cannot create endpoint"))
					Expect(hcsClient.CreateEndpointCallCount()).To(Equal(1))
				})
			})
		})

		Context("attaching the endpoint fails", func() {
			BeforeEach(func() {
				hcsClient.HotAttachEndpointReturns(errors.New("couldn't attach endpoint"))
			})

			It("deletes the endpoint and returns an error", func() {
				_, err := endpointManager.Create(nil)
				Expect(err).To(MatchError("couldn't attach endpoint"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				deletedEndpoint := hcsClient.DeleteEndpointArgsForCall(0)
				Expect(deletedEndpoint.Id).To(Equal(endpointId))
			})
		})
	})

	Describe("Delete", func() {
		var endpoint *hcsshim.HNSEndpoint

		BeforeEach(func() {
			endpoint = &hcsshim.HNSEndpoint{Id: endpointId}
			hcsClient.GetHNSEndpointByNameReturns(endpoint, nil)
		})

		It("detaches and deletes the endpoint", func() {
			Expect(endpointManager.Delete()).To(Succeed())

			Expect(hcsClient.HotDetachEndpointCallCount()).To(Equal(1))
			cId, eId := hcsClient.HotDetachEndpointArgsForCall(0)
			Expect(cId).To(Equal(containerId))
			Expect(eId).To(Equal(endpointId))

			Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
			Expect(hcsClient.DeleteEndpointArgsForCall(0)).To(Equal(endpoint))
		})

		Context("the endpoint doesn't exist", func() {
			BeforeEach(func() {
				hcsClient.GetHNSEndpointByNameReturns(nil, fmt.Errorf("Endpoint %s not found", containerId))
			})

			It("returns immediately without an error", func() {
				Expect(endpointManager.Delete()).To(Succeed())
				Expect(hcsClient.HotDetachEndpointCallCount()).To(Equal(0))
				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(0))
			})
		})

		Context("hns fails with some other error", func() {
			BeforeEach(func() {
				hcsClient.GetHNSEndpointByNameReturns(nil, errors.New("HNS fell over"))
			})

			It("returns an error", func() {
				Expect(endpointManager.Delete()).To(MatchError("HNS fell over"))
				Expect(hcsClient.HotDetachEndpointCallCount()).To(Equal(0))
				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(0))
			})
		})

		Context("the container doesn't exist", func() {
			BeforeEach(func() {
				hcsClient.HotDetachEndpointReturns(hcsshim.ErrComputeSystemDoesNotExist)
			})

			It("still deletes the endpoint", func() {
				Expect(endpointManager.Delete()).To(Succeed())
				Expect(hcsClient.HotDetachEndpointCallCount()).To(Equal(1))
				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				Expect(hcsClient.DeleteEndpointArgsForCall(0)).To(Equal(endpoint))
			})
		})

		Context("some other error occurs during the detach", func() {
			BeforeEach(func() {
				hcsClient.HotDetachEndpointReturns(errors.New("HNS crashed"))
			})

			It("returns an error but still deletes the endpoint", func() {
				Expect(endpointManager.Delete()).To(MatchError("HNS crashed"))
				Expect(hcsClient.HotDetachEndpointCallCount()).To(Equal(1))
				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				Expect(hcsClient.DeleteEndpointArgsForCall(0)).To(Equal(endpoint))
			})
		})

		Context("a delete error occurs", func() {
			BeforeEach(func() {
				hcsClient.DeleteEndpointReturns(nil, errors.New("couldn't delete endpoint"))
			})

			It("returns an error", func() {
				Expect(endpointManager.Delete()).To(MatchError("couldn't delete endpoint"))
			})
		})

		Context("both a detach error and a delete error occur", func() {
			BeforeEach(func() {
				hcsClient.HotDetachEndpointReturns(errors.New("detach failed"))
				hcsClient.DeleteEndpointReturns(nil, errors.New("couldn't delete endpoint"))
			})

			It("returns both errors", func() {
				Expect(endpointManager.Delete()).To(MatchError("detach failed, couldn't delete endpoint"))
			})
		})
	})
})
