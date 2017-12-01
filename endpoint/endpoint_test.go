package endpoint_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"code.cloudfoundry.org/winc/endpoint"
	"code.cloudfoundry.org/winc/endpoint/endpointfakes"
	"code.cloudfoundry.org/winc/netrules"
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
		netsh           *endpointfakes.FakeNetShRunner
	)

	BeforeEach(func() {
		hcsClient = &endpointfakes.FakeHCSClient{}
		netsh = &endpointfakes.FakeNetShRunner{}
		config := network.Config{
			NetworkName: networkName,
			DNSServers:  []string{"1.1.1.1", "2.2.2.2"},
		}

		endpointManager = endpoint.NewEndpointManager(hcsClient, netsh, containerId, config)

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

			Expect(hcsClient.HotAttachEndpointCallCount()).To(Equal(1))
			cId, eId := hcsClient.HotAttachEndpointArgsForCall(0)
			Expect(cId).To(Equal(containerId))
			Expect(eId).To(Equal(endpointId))

			Expect(netsh.RunHostCallCount()).To(Equal(1))
			args := netsh.RunHostArgsForCall(0)
			Expect(args).To(Equal([]string{"advfirewall", "firewall", "delete", "rule", "name=Compartment 9 - aaa-bbb"}))
		})

		Context("the network does not already exist", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturns(nil, fmt.Errorf("Network %s not found", networkName))
			})

			It("returns an error", func() {
				_, err := endpointManager.Create()
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

		Context("removing the firewall fails", func() {
			BeforeEach(func() {
				netsh.RunHostReturnsOnCall(0, []byte("couldn't delete"), errors.New("couldn't delete"))
			})

			It("deletes the endpoint and returns an error", func() {
				_, err := endpointManager.Create()
				Expect(err).To(MatchError("couldn't delete"))

				Expect(hcsClient.DeleteEndpointCallCount()).To(Equal(1))
				deletedEndpoint := hcsClient.DeleteEndpointArgsForCall(0)
				Expect(deletedEndpoint.Id).To(Equal(endpointId))
			})

			Context("when the error is 'No rules match the specified criteria'", func() {
				BeforeEach(func() {
					netsh.RunHostReturnsOnCall(0, []byte("No rules match the specified criteria."), errors.New("netsh failed: No rules match the specified criteria."))
					netsh.RunHostReturnsOnCall(1, []byte("No rules match the specified criteria."), errors.New("netsh failed: No rules match the specified criteria."))
					netsh.RunHostReturnsOnCall(2, []byte("OK"), nil)
				})

				It("retries creating the endpoint", func() {
					ep, err := endpointManager.Create()
					Expect(err).NotTo(HaveOccurred())
					Expect(ep.Id).To(Equal(endpointId))
				})
			})

			Context("it fails 3 times with 'No rules match the specified criteria'", func() {
				BeforeEach(func() {
					netsh.RunHostReturnsOnCall(0, []byte("No rules match the specified criteria."), errors.New("netsh failed: No rules match the specified criteria."))
					netsh.RunHostReturnsOnCall(1, []byte("No rules match the specified criteria."), errors.New("netsh failed: No rules match the specified criteria."))
					netsh.RunHostReturnsOnCall(2, []byte("No rules match the specified criteria."), errors.New("netsh failed: No rules match the specified criteria."))
				})

				It("returns an error", func() {
					_, err := endpointManager.Create()
					Expect(err).To(MatchError("netsh failed: No rules match the specified criteria."))
					Expect(netsh.RunHostCallCount()).To(Equal(3))
				})
			})
		})
	})

	Describe("ApplyMappings", func() {
		var (
			mapping1        netrules.PortMapping
			mapping2        netrules.PortMapping
			endpoint        hcsshim.HNSEndpoint
			updatedEndpoint hcsshim.HNSEndpoint
		)

		BeforeEach(func() {
			mapping1 = netrules.PortMapping{ContainerPort: 111, HostPort: 222}
			mapping2 = netrules.PortMapping{ContainerPort: 333, HostPort: 444}
			endpoint = hcsshim.HNSEndpoint{Id: endpointId}
			updatedEndpoint = hcsshim.HNSEndpoint{Id: endpointId, Policies: []json.RawMessage{[]byte("policies marshalled to json")}}
			hcsClient.UpdateEndpointReturns(&updatedEndpoint, nil)
		})

		It("updates the endpoint with the given port mappings", func() {
			ep, err := endpointManager.ApplyMappings(endpoint, []netrules.PortMapping{mapping1, mapping2})
			Expect(err).NotTo(HaveOccurred())
			Expect(ep).To(Equal(updatedEndpoint))

			Expect(hcsClient.UpdateEndpointCallCount()).To(Equal(1))
			endpointToUpdate := hcsClient.UpdateEndpointArgsForCall(0)
			Expect(endpointToUpdate.Id).To(Equal(endpointId))
			Expect(len(endpointToUpdate.Policies)).To(Equal(2))

			requestedPortMappings := []hcsshim.NatPolicy{}
			for _, pol := range endpointToUpdate.Policies {
				mapping := hcsshim.NatPolicy{}

				Expect(json.Unmarshal(pol, &mapping)).To(Succeed())
				requestedPortMappings = append(requestedPortMappings, mapping)
			}
			policies := []hcsshim.NatPolicy{
				{Type: "NAT", Protocol: "TCP", InternalPort: 111, ExternalPort: 222},
				{Type: "NAT", Protocol: "TCP", InternalPort: 333, ExternalPort: 444},
			}
			Expect(requestedPortMappings).To(ConsistOf(policies))
		})

		Context("no mappings are provided", func() {
			It("does not update the endpoint", func() {
				ep, err := endpointManager.ApplyMappings(endpoint, []netrules.PortMapping{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ep).To(Equal(endpoint))
				Expect(hcsClient.UpdateEndpointCallCount()).To(Equal(0))
			})
		})

		Context("updating the endpoint fails", func() {
			BeforeEach(func() {
				hcsClient.UpdateEndpointReturns(nil, errors.New("cannot update endpoint"))
			})

			It("does not retry", func() {
				_, err := endpointManager.ApplyMappings(endpoint, []netrules.PortMapping{mapping1, mapping2})
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
