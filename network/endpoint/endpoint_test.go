package endpoint_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/endpoint"
	"code.cloudfoundry.org/winc/network/endpoint/fakes"
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
		hcsClient       *fakes.HCSClient
		config          network.Config
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		config = network.Config{
			NetworkName: networkName,
			DNSServers:  []string{"1.1.1.1", "2.2.2.2"},
		}

		endpointManager = endpoint.NewEndpointManager(hcsClient, containerId, config)

		logrus.SetOutput(ioutil.Discard)
	})

	Describe("Create", func() {
		BeforeEach(func() {
			hcsClient.GetHNSNetworkByNameReturns(&hcsshim.HNSNetwork{Id: networkId, Name: networkName}, nil)
			hcsClient.CreateEndpointReturns(&hcsshim.HNSEndpoint{Id: endpointId}, nil)
			hcsClient.GetHNSEndpointByIDReturns(&hcsshim.HNSEndpoint{
				Id: endpointId,
			}, nil)
		})

		It("creates an endpoint on the configured network, attaches it to the container", func() {
			ep, err := endpointManager.Create()
			Expect(err).NotTo(HaveOccurred())
			Expect(ep.Id).To(Equal(endpointId))

			Expect(hcsClient.CreateEndpointCallCount()).To(Equal(1))
			endpointToCreate := hcsClient.CreateEndpointArgsForCall(0)
			Expect(endpointToCreate.VirtualNetwork).To(Equal(networkId))
			Expect(endpointToCreate.Name).To(Equal(containerId))
			Expect(endpointToCreate.DNSServerList).To(Equal("1.1.1.1,2.2.2.2"))
			Expect(endpointToCreate.Policies).To(BeEmpty())

			Expect(hcsClient.HotAttachEndpointCallCount()).To(Equal(1))
			cId, eId, _ := hcsClient.HotAttachEndpointArgsForCall(0)
			Expect(cId).To(Equal(containerId))
			Expect(eId).To(Equal(endpointId))
		})

		Context("the network config has MaximumOutgoingBandwidth set", func() {
			BeforeEach(func() {
				config.MaximumOutgoingBandwidth = 9988
				endpointManager = endpoint.NewEndpointManager(hcsClient, containerId, config)
			})

			It("adds a QOS policy with the correct bandwidth", func() {
				_, err := endpointManager.Create()
				Expect(err).NotTo(HaveOccurred())

				endpointToCreate := hcsClient.CreateEndpointArgsForCall(0)
				requestedPolicies := endpointToCreate.Policies
				Expect(len(requestedPolicies)).To(Equal(1))
				var qos hcsshim.QosPolicy
				Expect(json.Unmarshal(requestedPolicies[0], &qos)).To(Succeed())
				Expect(qos.Type).To(Equal(hcsshim.QOS))
				Expect(qos.MaximumOutgoingBandwidthInBytes).To(Equal(uint64(9988)))
			})
		})

		Context("the network does not already exist", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturns(nil, hcsshim.NetworkNotFoundError{NetworkName: networkName})
			})

			It("returns an error", func() {
				_, err := endpointManager.Create()
				Expect(err).To(BeAssignableToTypeOf(hcsshim.NetworkNotFoundError{}))
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
					ep, err := endpointManager.Create()
					Expect(err).NotTo(HaveOccurred())
					Expect(ep.Id).To(Equal(endpointId))
				})
			})

			Context("it fails 3 times with an unspecified HNS error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("HNS failed with error : Unspecified error"))
				})

				It("returns an error", func() {
					_, err := endpointManager.Create()
					Expect(err).To(MatchError("HNS failed with error : Unspecified error"))
					Expect(hcsClient.CreateEndpointCallCount()).To(Equal(3))
				})
			})

			Context("when the error is some other error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("cannot create endpoint"))
				})

				It("does not retry", func() {
					_, err := endpointManager.Create()
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
				_, err := endpointManager.Create()
				Expect(err).To(MatchError("couldn't attach endpoint"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				deletedEndpoint := hcsClient.DeleteEndpointArgsForCall(0)
				Expect(deletedEndpoint.Id).To(Equal(endpointId))
			})
		})

		Context("getting the allocated endpoint fails", func() {
			BeforeEach(func() {
				hcsClient.GetHNSEndpointByIDReturns(nil, errors.New("couldn't load"))
			})

			It("deletes the endpoint and returns an error", func() {
				_, err := endpointManager.Create()
				Expect(err).To(MatchError("couldn't load"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				deletedEndpoint := hcsClient.DeleteEndpointArgsForCall(0)
				Expect(deletedEndpoint.Id).To(Equal(endpointId))
			})
		})
	})

	Describe("ApplyPolicies", func() {
		var (
			nat1            *hcsshim.NatPolicy
			nat2            *hcsshim.NatPolicy
			acl1            *hcsshim.ACLPolicy
			acl2            *hcsshim.ACLPolicy
			endpoint        hcsshim.HNSEndpoint
			updatedEndpoint hcsshim.HNSEndpoint
		)

		BeforeEach(func() {
			nat1 = &hcsshim.NatPolicy{Type: hcsshim.Nat, Protocol: "TCP", InternalPort: 111, ExternalPort: 222}
			nat2 = &hcsshim.NatPolicy{Type: hcsshim.Nat, Protocol: "TCP", InternalPort: 333, ExternalPort: 444}

			acl1 = &hcsshim.ACLPolicy{Type: hcsshim.ACL, Direction: hcsshim.In, Action: hcsshim.Allow, LocalPorts: "111"}
			acl2 = &hcsshim.ACLPolicy{Type: hcsshim.ACL, Direction: hcsshim.In, Action: hcsshim.Allow, LocalPorts: "333"}

			endpoint = hcsshim.HNSEndpoint{
				Id:       endpointId,
				Policies: []json.RawMessage{[]byte("existing policy")},
			}
			updatedEndpoint = hcsshim.HNSEndpoint{
				Id:       endpointId,
				Policies: []json.RawMessage{[]byte("policies marshalled to json")},
			}
			hcsClient.UpdateEndpointReturns(&updatedEndpoint, nil)
			hcsClient.GetHNSEndpointByIDReturns(&updatedEndpoint, nil)
		})

		It("updates the endpoint with the given port mappings", func() {
			ep, err := endpointManager.ApplyPolicies(endpoint, []*hcsshim.NatPolicy{nat1, nat2}, []*hcsshim.ACLPolicy{acl1, acl2})
			Expect(err).NotTo(HaveOccurred())
			Expect(ep).To(Equal(updatedEndpoint))

			Expect(hcsClient.UpdateEndpointCallCount()).To(Equal(1))
			endpointToUpdate := hcsClient.UpdateEndpointArgsForCall(0)
			Expect(endpointToUpdate.Id).To(Equal(endpointId))
			Expect(len(endpointToUpdate.Policies)).To(Equal(5))
			Expect(endpointToUpdate.Policies[0]).To(Equal(json.RawMessage("existing policy")))

			requestedNats := []hcsshim.NatPolicy{}
			requestedAcls := []hcsshim.ACLPolicy{}

			for _, pol := range endpointToUpdate.Policies[1:] {
				p := hcsshim.Policy{}
				nat := hcsshim.NatPolicy{}
				acl := hcsshim.ACLPolicy{}

				Expect(json.Unmarshal(pol, &p)).To(Succeed())

				if p.Type == hcsshim.Nat {
					Expect(json.Unmarshal(pol, &nat)).To(Succeed())
					requestedNats = append(requestedNats, nat)
				}

				if p.Type == hcsshim.ACL {
					Expect(json.Unmarshal(pol, &acl)).To(Succeed())
					requestedAcls = append(requestedAcls, acl)
				}
			}
			expectedNats := []hcsshim.NatPolicy{
				{Type: "NAT", Protocol: "TCP", InternalPort: 111, ExternalPort: 222},
				{Type: "NAT", Protocol: "TCP", InternalPort: 333, ExternalPort: 444},
			}
			Expect(requestedNats).To(ConsistOf(expectedNats))

			expectedAcls := []hcsshim.ACLPolicy{
				{Type: hcsshim.ACL, Direction: hcsshim.In, Action: hcsshim.Allow, LocalPorts: "111"},
				{Type: hcsshim.ACL, Direction: hcsshim.In, Action: hcsshim.Allow, LocalPorts: "333"},
			}
			Expect(requestedAcls).To(ConsistOf(expectedAcls))
		})

		Context("no HNS ACLs are provided", func() {
			It("generates default block all ACL policies", func() {
				ep, err := endpointManager.ApplyPolicies(endpoint, []*hcsshim.NatPolicy{nat1, nat2}, []*hcsshim.ACLPolicy{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ep).To(Equal(updatedEndpoint))

				endpointToUpdate := hcsClient.UpdateEndpointArgsForCall(0)
				Expect(len(endpointToUpdate.Policies)).To(Equal(5))
				Expect(endpointToUpdate.Policies[0]).To(Equal(json.RawMessage("existing policy")))

				requestedAcls := []hcsshim.ACLPolicy{}

				for _, pol := range endpointToUpdate.Policies[1:] {
					p := hcsshim.Policy{}
					acl := hcsshim.ACLPolicy{}

					Expect(json.Unmarshal(pol, &p)).To(Succeed())

					if p.Type == hcsshim.ACL {
						Expect(json.Unmarshal(pol, &acl)).To(Succeed())
						requestedAcls = append(requestedAcls, acl)
					}
				}

				expectedAcls := []hcsshim.ACLPolicy{
					{Type: hcsshim.ACL, Direction: hcsshim.In, Action: hcsshim.Block, Protocol: 256},
					{Type: hcsshim.ACL, Direction: hcsshim.Out, Action: hcsshim.Block, Protocol: 256},
				}
				Expect(requestedAcls).To(ConsistOf(expectedAcls))
			})
		})

		Context("no HNS Nat policies are provided", func() {
			It("still updates the endpoint", func() {
				ep, err := endpointManager.ApplyPolicies(endpoint, []*hcsshim.NatPolicy{}, []*hcsshim.ACLPolicy{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ep).To(Equal(updatedEndpoint))

				Expect(hcsClient.UpdateEndpointCallCount()).To(Equal(1))
			})
		})

		Context("updating the endpoint fails", func() {
			BeforeEach(func() {
				hcsClient.UpdateEndpointReturns(nil, errors.New("cannot update endpoint"))
			})

			It("does not retry", func() {
				_, err := endpointManager.ApplyPolicies(endpoint, []*hcsshim.NatPolicy{}, []*hcsshim.ACLPolicy{})
				Expect(err).To(MatchError("cannot update endpoint"))
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
				hcsClient.GetHNSEndpointByNameReturns(nil, hcsshim.EndpointNotFoundError{EndpointName: containerId})
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
