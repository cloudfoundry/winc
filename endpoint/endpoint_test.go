package endpoint_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"net"

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
			natPolicies     []*hcsshim.NatPolicy
			aclPolicies     []*hcsshim.ACLPolicy
			createdEndpoint hcsshim.HNSEndpoint
		)

		BeforeEach(func() {
			natPolicies = []*hcsshim.NatPolicy{{InternalPort: 111, ExternalPort: 222}}
			aclPolicies = []*hcsshim.ACLPolicy{{LocalPort: 333}}

			hcsClient.GetHNSNetworkByNameReturns(&hcsshim.HNSNetwork{Id: networkId, Name: networkName}, nil)

			createdEndpoint = hcsshim.HNSEndpoint{Id: endpointId, DNSServerList: "dns-servers-from-container", IPAddress: net.ParseIP("11.22.33.44")}
			hcsClient.CreateEndpointReturns(&createdEndpoint, nil)
		})

		It("creates an endpoint on the configured network and attaches it to the container", func() {
			Expect(endpointManager.Create(natPolicies, aclPolicies)).To(Succeed())

			Expect(hcsClient.CreateEndpointCallCount()).To(Equal(1))
			endpointToCreate := hcsClient.CreateEndpointArgsForCall(0)
			Expect(endpointToCreate.VirtualNetwork).To(Equal(networkId))
			Expect(endpointToCreate.Name).To(Equal(containerId))
			Expect(len(endpointToCreate.Policies)).To(Equal(1))
			Expect(endpointToCreate.DNSServerList).To(Equal("1.1.1.1,2.2.2.2"))

			requestedNatPolicies := []hcsshim.NatPolicy{}
			for _, natPolicy := range endpointToCreate.Policies {
				mapping := hcsshim.NatPolicy{}

				Expect(json.Unmarshal(natPolicy, &mapping)).To(Succeed())
				requestedNatPolicies = append(requestedNatPolicies, mapping)
			}
			Expect(len(requestedNatPolicies)).To(Equal(len(natPolicies)))
			Expect(requestedNatPolicies[0]).To(Equal(*natPolicies[0]))

			Expect(hcsClient.HotAttachEndpointCallCount()).To(Equal(1))
			cId, eId := hcsClient.HotAttachEndpointArgsForCall(0)
			Expect(cId).To(Equal(containerId))
			Expect(eId).To(Equal(endpointId))

			actualEndpoint, actualACLPolicies := hcsClient.ApplyACLPolicyArgsForCall(0)
			Expect(*actualEndpoint).To(Equal(createdEndpoint))
			Expect(actualACLPolicies).To(ContainElement(aclPolicies[0]))
			blockInACLPolicy := hcsshim.ACLPolicy{
				Protocol:       netrules.WindowsProtocolTCP,
				Type:           hcsshim.ACL,
				Action:         hcsshim.Block,
				Direction:      hcsshim.In,
				LocalAddresses: "11.22.33.44",
			}

			blockOutACLPolicy := hcsshim.ACLPolicy{
				Protocol:       netrules.WindowsProtocolTCP,
				Type:           hcsshim.ACL,
				Action:         hcsshim.Block,
				Direction:      hcsshim.Out,
				LocalAddresses: "11.22.33.44",
			}
			foundIn := false
			foundOut := false
			for _, v := range actualACLPolicies {
				if *v == blockInACLPolicy {
					foundIn = true
				}

				if *v == blockOutACLPolicy {
					foundOut = true
				}
			}
			Expect(foundOut).To(BeTrue())
			Expect(foundIn).To(BeTrue())
		})

		Context("the network does not already exist", func() {
			BeforeEach(func() {
				hcsClient.GetHNSNetworkByNameReturns(nil, fmt.Errorf("Network %s not found", networkName))
			})

			It("returns an error", func() {
				err := endpointManager.Create(nil, nil)
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
					err := endpointManager.Create(nil, nil)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("it fails 3 times with an unspecified HNS error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("HNS failed with error : Unspecified error"))
				})

				It("returns an error", func() {
					err := endpointManager.Create(nil, nil)
					Expect(err).To(MatchError("HNS failed with error : Unspecified error"))
					Expect(hcsClient.CreateEndpointCallCount()).To(Equal(3))
				})
			})

			Context("when the error is some other error", func() {
				BeforeEach(func() {
					hcsClient.CreateEndpointReturns(nil, errors.New("cannot create endpoint"))
				})

				It("does not retry", func() {
					err := endpointManager.Create(nil, nil)
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
				err := endpointManager.Create(nil, nil)
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
