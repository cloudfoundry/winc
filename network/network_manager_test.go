package network_test

import (
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
	)

	BeforeEach(func() {
		hcsClient = &networkfakes.FakeHCSClient{}
		netRuleApplier = &networkfakes.FakeNetRuleApplier{}
		config = network.Config{
			MTU: 1434,
		}

		networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, config, containerId, false)

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
			eId, mtu := netRuleApplier.MTUArgsForCall(0)
			Expect(eId).To(Equal("ep-987"))
			Expect(mtu).To(Equal(1434))
		})

		Context("when run on a insider preview", func() {
			BeforeEach(func() {
				networkManager = network.NewNetworkManager(hcsClient, netRuleApplier, config, containerId, true)
			})

			It("sets the MTU using the container ID", func() {
				_, err := networkManager.Up(inputs)
				Expect(err).NotTo(HaveOccurred())

				Expect(netRuleApplier.MTUCallCount()).To(Equal(1))
				actualContainerId, actualMtu := netRuleApplier.MTUArgsForCall(0)
				Expect(actualContainerId).To(Equal(containerId))
				Expect(actualMtu).To(Equal(1434))
			})
		})
	})

	Describe("Down", func() {
		It("cleans up the container network", func() {
			Expect(networkManager.Down()).To(Succeed())
			Expect(netRuleApplier.CleanupCallCount()).To(Equal(1))
		})
	})
})
