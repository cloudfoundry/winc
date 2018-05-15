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
		firewall        *fakes.Firewall
	)

	BeforeEach(func() {
		hcsClient = &fakes.HCSClient{}
		firewall = &fakes.Firewall{}
		config = network.Config{
			NetworkName: networkName,
			DNSServers:  []string{"1.1.1.1", "2.2.2.2"},
		}

		endpointManager = endpoint.NewEndpointManager(hcsClient, firewall, containerId, config)

		logrus.SetOutput(ioutil.Discard)
	})

	Describe("Create", func() {
		BeforeEach(func() {
			hcsClient.GetHNSNetworkByNameReturns(&hcsshim.HNSNetwork{Id: networkId, Name: networkName}, nil)
			hcsClient.CreateEndpointReturns(&hcsshim.HNSEndpoint{Id: endpointId}, nil)
			hcsClient.GetHNSEndpointByIDReturns(&hcsshim.HNSEndpoint{
				Id: endpointId, Resources: hcsshim.Resources{
					Allocators: []hcsshim.Allocator{{CompartmentId: 9, EndpointPortGuid: "aaa-bbb", Type: hcsshim.EndpointPortType}, {Type: 5}},
				},
			}, nil)

			firewall.RuleExistsReturnsOnCall(0, true, nil)
		})

		It("creates an endpoint on the configured network, attaches it to the container, and deletes the compartment firewall rule", func() {
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

			Expect(firewall.DeleteRuleCallCount()).To(Equal(1))
			Expect(firewall.DeleteRuleArgsForCall(0)).To(Equal("Compartment 9 - aaa-bbb"))
		})

		Context("the network config has MaximumOutgoingBandwidth set", func() {
			BeforeEach(func() {
				config.MaximumOutgoingBandwidth = 9988
				endpointManager = endpoint.NewEndpointManager(hcsClient, firewall, containerId, config)
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

		Context("the allocated endpoint does not return an EndpointPort allocator", func() {
			BeforeEach(func() {
				hcsClient.GetHNSEndpointByIDReturns(&hcsshim.HNSEndpoint{
					Id: endpointId, Resources: hcsshim.Resources{
						Allocators: []hcsshim.Allocator{{Type: 5}},
					},
				}, nil)
			})

			It("deletes the endpoint and returns an error", func() {
				_, err := endpointManager.Create()
				Expect(err.Error()).To(ContainSubstring("invalid endpoint endpointid-abcd allocators"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				deletedEndpoint := hcsClient.DeleteEndpointArgsForCall(0)
				Expect(deletedEndpoint.Id).To(Equal(endpointId))
			})
		})

		Context("checking existance of the firewall rule fails", func() {
			BeforeEach(func() {
				firewall.RuleExistsReturnsOnCall(0, false, errors.New("couldn't check"))
			})

			It("returns an error and deletes the endpoint", func() {
				_, err := endpointManager.Create()
				Expect(err).To(MatchError("couldn't check"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				deletedEndpoint := hcsClient.DeleteEndpointArgsForCall(0)
				Expect(deletedEndpoint.Id).To(Equal(endpointId))
			})
		})

		Context("removing the firewall fails", func() {
			BeforeEach(func() {
				firewall.DeleteRuleReturnsOnCall(0, errors.New("couldn't delete"))
			})

			It("deletes the endpoint and returns an error", func() {
				_, err := endpointManager.Create()
				Expect(err).To(MatchError("couldn't delete"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				deletedEndpoint := hcsClient.DeleteEndpointArgsForCall(0)
				Expect(deletedEndpoint.Id).To(Equal(endpointId))
			})
		})

		Context("the firewall rule takes a while to be created", func() {
			Context("the rule exists by the 3rd check", func() {
				BeforeEach(func() {
					firewall.RuleExistsReturnsOnCall(0, false, nil)
					firewall.RuleExistsReturnsOnCall(1, false, nil)
					firewall.RuleExistsReturnsOnCall(2, true, nil)
				})

				It("it removes the firewall rule", func() {
					_, err := endpointManager.Create()
					Expect(err).NotTo(HaveOccurred())

					Expect(firewall.DeleteRuleCallCount()).To(Equal(1))
					Expect(firewall.DeleteRuleArgsForCall(0)).To(Equal("Compartment 9 - aaa-bbb"))
				})
			})

			Context("the firewall rule does not exist by the 3rd check", func() {
				BeforeEach(func() {
					firewall.RuleExistsReturnsOnCall(0, false, nil)
					firewall.RuleExistsReturnsOnCall(1, false, nil)
					firewall.RuleExistsReturnsOnCall(2, false, nil)
				})

				It("returns an error and doesn't delete the rule", func() {
					_, err := endpointManager.Create()
					Expect(err).To(MatchError("firewall rule Compartment 9 - aaa-bbb not generated in time"))

					Expect(firewall.DeleteRuleCallCount()).To(Equal(0))
					Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
					deletedEndpoint := hcsClient.DeleteEndpointArgsForCall(0)
					Expect(deletedEndpoint.Id).To(Equal(endpointId))
				})
			})
		})
	})

	Describe("ApplyPolicies", func() {
		var (
			nat1            hcsshim.NatPolicy
			nat2            hcsshim.NatPolicy
			endpoint        hcsshim.HNSEndpoint
			updatedEndpoint hcsshim.HNSEndpoint
		)

		BeforeEach(func() {
			nat1 = hcsshim.NatPolicy{Type: hcsshim.Nat, Protocol: "TCP", InternalPort: 111, ExternalPort: 222}
			nat2 = hcsshim.NatPolicy{Type: hcsshim.Nat, Protocol: "TCP", InternalPort: 333, ExternalPort: 444}
			endpoint = hcsshim.HNSEndpoint{
				Id:       endpointId,
				Policies: []json.RawMessage{[]byte("existing policy")},
			}
			updatedEndpoint = hcsshim.HNSEndpoint{
				Id:       endpointId,
				Policies: []json.RawMessage{[]byte("policies marshalled to json")},
				Resources: hcsshim.Resources{
					Allocators: []hcsshim.Allocator{
						{Type: hcsshim.NATPolicyType},
					},
				},
			}
			hcsClient.UpdateEndpointReturns(&updatedEndpoint, nil)
			hcsClient.GetHNSEndpointByIDReturns(&updatedEndpoint, nil)
		})

		It("updates the endpoint with the given port mappings", func() {
			ep, err := endpointManager.ApplyPolicies(endpoint, []hcsshim.NatPolicy{nat1, nat2}, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(ep).To(Equal(updatedEndpoint))

			Expect(hcsClient.UpdateEndpointCallCount()).To(Equal(1))
			endpointToUpdate := hcsClient.UpdateEndpointArgsForCall(0)
			Expect(endpointToUpdate.Id).To(Equal(endpointId))
			Expect(len(endpointToUpdate.Policies)).To(Equal(3))
			Expect(endpointToUpdate.Policies[0]).To(Equal(json.RawMessage("existing policy")))

			requestedPortMappings := []hcsshim.NatPolicy{}
			for _, pol := range endpointToUpdate.Policies[1:] {
				mapping := hcsshim.NatPolicy{}

				Expect(json.Unmarshal(pol, &mapping)).To(Succeed())
				requestedPortMappings = append(requestedPortMappings, mapping)
			}
			policies := []hcsshim.NatPolicy{
				{Type: "NAT", Protocol: "TCP", InternalPort: 111, ExternalPort: 222},
				{Type: "NAT", Protocol: "TCP", InternalPort: 333, ExternalPort: 444},
			}
			Expect(requestedPortMappings).To(ConsistOf(policies))

			Expect(hcsClient.GetHNSEndpointByIDCallCount()).To(Equal(1))
		})

		Context("no mappings are provided", func() {
			It("does not update the endpoint", func() {
				ep, err := endpointManager.ApplyPolicies(endpoint, []hcsshim.NatPolicy{}, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(ep).To(Equal(endpoint))
				Expect(hcsClient.UpdateEndpointCallCount()).To(Equal(0))
			})
		})

		Context("nat initialization takes too long", func() {
			BeforeEach(func() {
				hcsClient.GetHNSEndpointByIDReturns(&hcsshim.HNSEndpoint{
					Resources: hcsshim.Resources{
						Allocators: []hcsshim.Allocator{
							{Type: 10},
						},
					},
				}, nil)
			})

			It("errors after repeatedly checking if the endpoint is ready", func() {
				_, err := endpointManager.ApplyPolicies(endpoint, []hcsshim.NatPolicy{nat1, nat2}, nil)
				Expect(err).To(MatchError("NAT not initialized in time"))
				Expect(hcsClient.GetHNSEndpointByIDCallCount()).To(Equal(10))
			})
		})

		Context("updating the endpoint fails", func() {
			BeforeEach(func() {
				hcsClient.UpdateEndpointReturns(nil, errors.New("cannot update endpoint"))
			})

			It("does not retry", func() {
				_, err := endpointManager.ApplyPolicies(endpoint, []hcsshim.NatPolicy{nat1, nat2}, nil)
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
