package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network/netinterface"
	"code.cloudfoundry.org/winc/network/netrules"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Up", func() {
	var (
		bundleSpec specs.Spec
		n          netinterface.NetInterface
		localIp    string
	)

	BeforeEach(func() {
		bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
		n = netinterface.NetInterface{}

		var err error
		localIp, err = localip.LocalIP()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		failed = failed || CurrentGinkgoTestDescription().Failed
	})

	Context("default network config", func() {
		BeforeEach(func() {
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
			networkConfig = helpers.GenerateNetworkConfig()
			helpers.CreateNetwork(networkConfig, networkConfigFile)
		})

		AfterEach(func() {
			deleteContainerAndNetwork(containerId, networkConfig)
		})

		It("sets the host MTU in the container", func() {
			helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`, networkConfigFile)

			containerMtu, err := n.GetMTU(fmt.Sprintf("vEthernet (%s)", containerId), windows.AF_INET)
			Expect(err).ToNot(HaveOccurred())

			hostAdapter, err := n.ByIP(localIp)
			Expect(err).ToNot(HaveOccurred())

			hostMtu, err := n.GetMTU(hostAdapter.Name, windows.AF_INET)
			Expect(err).ToNot(HaveOccurred())

			Expect(containerMtu).To(Equal(hostMtu))
		})

		Context("stdin contains a net in rule", func() {
			var (
				hostPort1      uint32
				hostPort2      uint32
				containerPort1 uint32
				containerPort2 uint32
				client         http.Client
			)

			BeforeEach(func() {
				hostPort1 = 0
				hostPort2 = uint32(randomPort())

				containerPort1 = 12345
				containerPort2 = 9876

				client = *http.DefaultClient
				client.Timeout = 5 * time.Second

				pid := helpers.GetContainerState(containerId).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

				_, _, err := helpers.ExecInContainer(containerId, []string{"c:\\server.exe", strconv.Itoa(int(containerPort1))}, true)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = helpers.ExecInContainer(containerId, []string{"c:\\server.exe", strconv.Itoa(int(containerPort2))}, true)
				Expect(err).NotTo(HaveOccurred())
			})

			It("generates the correct port mappings and binds them to the container", func() {
				outputs := helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %d},{"host_port": %d, "container_port": %d}]}`, hostPort1, containerPort1, hostPort2, containerPort2), networkConfigFile)

				mappedPorts := []netrules.PortMapping{}
				Expect(json.Unmarshal([]byte(outputs.Properties.MappedPorts), &mappedPorts)).To(Succeed())

				Expect(len(mappedPorts)).To(Equal(2))

				Expect(mappedPorts[0].ContainerPort).To(Equal(containerPort1))
				Expect(mappedPorts[0].HostPort).NotTo(Equal(hostPort1))

				Expect(mappedPorts[1].ContainerPort).To(Equal(containerPort2))
				Expect(mappedPorts[1].HostPort).To(Equal(hostPort2))

				hostPort1 = mappedPorts[0].HostPort

				hostIp, err := localip.LocalIP()
				Expect(err).NotTo(HaveOccurred())

				resp, err := client.Get(fmt.Sprintf("http://%s:%d", hostIp, hostPort1))
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				data, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %d", containerPort1)))

				resp2, err := client.Get(fmt.Sprintf("http://%s:%d", hostIp, hostPort2))
				Expect(err).NotTo(HaveOccurred())
				defer resp2.Body.Close()

				data, err = ioutil.ReadAll(resp2.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %d", containerPort2)))
			})

			It("can hit a port on the container directly", func() {
				helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %d},{"host_port": %d, "container_port": %d}]}`, hostPort1, containerPort1, hostPort2, containerPort2), networkConfigFile)

				resp, err := client.Get(fmt.Sprintf("http://%s:%d", getContainerIp(containerId), containerPort1))
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				data, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %d", containerPort1)))

				resp2, err := client.Get(fmt.Sprintf("http://%s:%d", getContainerIp(containerId), containerPort2))
				Expect(err).NotTo(HaveOccurred())
				defer resp2.Body.Close()

				data, err = ioutil.ReadAll(resp2.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %d", containerPort2)))
			})

			It("creates the correct urlacl in the container", func() {
				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`, networkConfigFile)

				stdout, _, err := helpers.ExecInContainer(containerId, []string{"cmd.exe", "/C", "netsh http show urlacl url=http://*:8080/ | findstr User"}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(stdout.String()).To(ContainSubstring("BUILTIN\\Users"))
			})

			Context("stdin does not contain a port mapping request", func() {
				It("cannot listen on any ports", func() {
					helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} }`, networkConfigFile)

					_, err := client.Get(fmt.Sprintf("http://%s:%d", getContainerIp(containerId), containerPort1))
					Expect(err).To(HaveOccurred())
					errorMsg := fmt.Sprintf("Get http://%s:%d: net/http: request canceled while waiting for connection", getContainerIp(containerId), containerPort1)
					Expect(err.Error()).To(ContainSubstring(errorMsg))

					_, err = client.Get(fmt.Sprintf("http://%s:%d", getContainerIp(containerId), containerPort2))
					Expect(err).To(HaveOccurred())
					errorMsg = fmt.Sprintf("Get http://%s:%d: net/http: request canceled while waiting for connection", getContainerIp(containerId), containerPort2)
					Expect(err.Error()).To(ContainSubstring(errorMsg))
				})

				It("prints an empty list of mapped ports", func() {
					outputs := helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} }`, networkConfigFile)

					Expect(outputs.Properties.MappedPorts).To(Equal("[]"))
					Expect(outputs.Properties.DeprecatedHostIP).To(Equal("255.255.255.255"))

					_, network, err := net.ParseCIDR(networkConfig.SubnetRange)
					Expect(err).NotTo(HaveOccurred())
					ip := net.ParseIP(outputs.Properties.ContainerIP)
					Expect(ip).NotTo(BeNil())
					Expect(network.Contains(ip)).To(BeTrue())
				})
			})
		})

		Context("stdin does not contain net out rules", func() {
			BeforeEach(func() {
				pid := helpers.GetContainerState(containerId).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)
			})

			It("cannot resolve DNS", func() {
				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

				stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "dns", "--addr", "www.google.com"}, false)
				Expect(err).To(HaveOccurred())

				Expect(stdout.String()).To(ContainSubstring("lookup www.google.com: no such host"))
			})

			It("cannot connect to a remote host over TCP", func() {
				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

				stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
				Expect(err).To(HaveOccurred())

				errStr := "dial tcp 8.8.8.8:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
				Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))
			})

			It("cannot connect to a remote host over UDP", func() {

				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

				stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53"}, false)
				Expect(err).To(HaveOccurred())

				Expect(stdout.String()).To(ContainSubstring("failed to exchange: read udp"))
				Expect(stdout.String()).To(ContainSubstring("8.8.8.8:53: i/o timeout"))
			})

			It("cannot connect to a remote host over ICMP", func() {
				if windowsBuild == 16299 {
					Skip("ping.exe elevates to admin, breaking this test on 1709")
				}

				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

				stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
				Expect(err).To(HaveOccurred())

				Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.8.8"))
				Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
			})
		})

		Context("stdin contains net out rules", func() {
			var (
				netOutRules []byte
				netOutRule  netrules.NetOut
			)

			BeforeEach(func() {
				netOutRule = netrules.NetOut{
					Networks: []netrules.IPRange{
						{Start: net.ParseIP("8.8.5.5"), End: net.ParseIP("9.0.0.0")},
					},
					Ports: []netrules.PortRange{{Start: 40, End: 60}},
				}

				pid := helpers.GetContainerState(containerId).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)
			})

			Context("netout allows udp", func() {
				BeforeEach(func() {
					var err error

					netOutRule.Protocol = netrules.ProtocolUDP
					netOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
					Expect(err).NotTo(HaveOccurred())
				})

				It("can connect to a remote host over UDP", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).NotTo(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("recieved response to DNS query from 8.8.8.8:53 over UDP"))
				})

				It("cannot connect to a remote host over TCP", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())

					errStr := "dial tcp 8.8.8.8:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
					Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))
				})

				It("cannot connect to a remote host over ICMP", func() {
					if windowsBuild == 16299 {
						Skip("ping.exe elevates to admin, breaking this test on 1709")
					}

					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
					Expect(err).To(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.8.8"))
					Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
				})

				It("cannot connect to a remote host over UDP prohibited by netout", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.4.4", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("failed to exchange: read udp"))
					Expect(stdout.String()).To(ContainSubstring("8.8.4.4:53: i/o timeout"))
				})

				Context("netout allows udp on port 53", func() {
					BeforeEach(func() {
						var err error

						netOutRule.Networks = []netrules.IPRange{
							{Start: net.ParseIP("0.0.0.0"), End: net.ParseIP("255.255.255.255")},
						}
						netOutRule.Ports = []netrules.PortRange{{Start: 53, End: 53}}

						netOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
						Expect(err).NotTo(HaveOccurred())
					})

					It("can resolve DNS", func() {
						helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

						stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "dns", "--addr", "www.google.com"}, false)
						Expect(err).NotTo(HaveOccurred())

						Expect(stdout.String()).To(ContainSubstring("found addr"))
					})
				})
			})

			Context("netout allows tcp", func() {
				BeforeEach(func() {
					var err error

					netOutRule.Protocol = netrules.ProtocolTCP
					netOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
					Expect(err).NotTo(HaveOccurred())
				})

				It("can connect to a remote host over TCP", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).NotTo(HaveOccurred())

					Expect(strings.TrimSpace(stdout.String())).To(Equal("connected to 8.8.8.8:53 over tcp"))
				})

				It("cannot connect to a remote host over UDP", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("failed to exchange: read udp"))
					Expect(stdout.String()).To(ContainSubstring("8.8.8.8:53: i/o timeout"))
				})

				It("cannot connect to a remote host over ICMP", func() {
					if windowsBuild == 16299 {
						Skip("ping.exe elevates to admin, breaking this test on 1709")
					}

					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
					Expect(err).To(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.8.8"))
					Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
				})

				It("cannot connect to a remote server over TCP prohibited by netout", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.4.4", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())

					errStr := "dial tcp 8.8.4.4:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
					Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))
				})
			})

			Context("netout allows icmp", func() {
				BeforeEach(func() {
					var err error

					netOutRule.Protocol = netrules.ProtocolICMP
					netOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
					Expect(err).NotTo(HaveOccurred())
				})

				It("can connect to a remote host over ICMP", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
					Expect(err).NotTo(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.8.8"))
					Expect(stdout.String()).NotTo(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
					Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received ="))
				})

				It("cannot connect to a remote host over TCP", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())

					errStr := "dial tcp 8.8.8.8:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
					Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))
				})

				It("cannot connect to a remote host over UDP", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("failed to exchange: read udp"))
					Expect(stdout.String()).To(ContainSubstring("8.8.8.8:53: i/o timeout"))
				})

				It("cannot connect to a remote host over ICMP prohibited by netout", func() {
					if windowsBuild == 16299 {
						Skip("ping.exe elevates to admin, breaking this test on 1709")
					}

					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.4.4"}, false)
					Expect(err).To(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.4.4"))
					Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
				})
			})

			Context("netout allows all", func() {
				BeforeEach(func() {
					var err error

					netOutRule.Protocol = netrules.ProtocolAll
					netOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
					Expect(err).NotTo(HaveOccurred())
				})

				It("allows access over all protocols to valid remote hosts", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(stdout.String()).To(ContainSubstring("recieved response to DNS query from 8.8.8.8:53 over UDP"))

					stdout, _, err = helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(strings.TrimSpace(stdout.String())).To(Equal("connected to 8.8.8.8:53 over tcp"))

					stdout, _, err = helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.8.8"))
					Expect(stdout.String()).To(ContainSubstring("Reply from 8.8.8.8: bytes=32"))
					Expect(stdout.String()).NotTo(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
				})

				It("blocks access over all protocols to prohibited remote hosts", func() {
					helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

					stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.4.4", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())
					Expect(stdout.String()).To(ContainSubstring("failed to exchange: read udp"))
					Expect(stdout.String()).To(ContainSubstring("8.8.4.4:53: i/o timeout"))

					stdout, _, err = helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.4.4", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())
					errStr := "dial tcp 8.8.4.4:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
					Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))

					if windowsBuild != 16299 {
						// this test works on 1803

						stdout, _, err = helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.4.4"}, false)
						Expect(err).To(HaveOccurred())
						Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.4.4"))
						Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
					}
				})
			})
		})

	})

	Context("custom MTU", func() {
		BeforeEach(func() {
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
			networkConfig = helpers.GenerateNetworkConfig()
			networkConfig.MTU = 1405
			helpers.CreateNetwork(networkConfig, networkConfigFile)

		})

		AfterEach(func() {
			deleteContainerAndNetwork(containerId, networkConfig)
		})

		It("sets the network MTU on the internal container NIC", func() {
			helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`, networkConfigFile)

			containerMtu, err := n.GetMTU(fmt.Sprintf("vEthernet (%s)", containerId), windows.AF_INET)
			Expect(err).ToNot(HaveOccurred())

			Expect(containerMtu).To(Equal(uint32(1405)))
		})
	})

	Context("custom DNS Servers", func() {
		BeforeEach(func() {
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
			networkConfig = helpers.GenerateNetworkConfig()
			networkConfig.DNSServers = []string{"8.8.8.8", "8.8.4.4"}
			helpers.CreateNetwork(networkConfig, networkConfigFile)
		})

		AfterEach(func() {
			deleteContainerAndNetwork(containerId, networkConfig)
		})

		It("uses those IP addresses as DNS servers", func() {
			helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`, networkConfigFile)

			stdout, _, err := helpers.ExecInContainer(containerId, []string{"powershell.exe", "-Command", `(Get-DnsClientServerAddress -InterfaceAlias 'vEthernet*' -AddressFamily IPv4).ServerAddresses -join ","`}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(stdout.String())).To(Equal("8.8.8.8,8.8.4.4"))
		})

		It("allows traffic to those servers", func() {
			helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`, networkConfigFile)

			pid := helpers.GetContainerState(containerId).Pid
			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)

			stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(stdout.String())).To(Equal("connected to 8.8.8.8:53 over tcp"))
		})
	})

	Context("two containers are running", func() {
		var (
			bundlePath2   string
			bundleSpec2   specs.Spec
			containerId2  string
			containerPort string
			hostIP        string
			client        http.Client
		)

		BeforeEach(func() {
			var err error

			bundlePath2, err = ioutil.TempDir("", "winccontainer-2")
			Expect(err).NotTo(HaveOccurred())
			containerId2 = filepath.Base(bundlePath2)

			bundleSpec2 = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId2))

			containerPort = "12345"

			hostIP, err = localip.LocalIP()
			Expect(err).NotTo(HaveOccurred())

			client = *http.DefaultClient
			client.Timeout = 5 * time.Second

			networkConfig = helpers.GenerateNetworkConfig()
			helpers.CreateNetwork(networkConfig, networkConfigFile)
		})

		AfterEach(func() {
			helpers.NetworkDown(containerId2, networkConfigFile)
			helpers.DeleteContainer(containerId2)
			helpers.DeleteVolume(containerId2)
			deleteContainerAndNetwork(containerId, networkConfig)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
			Expect(os.RemoveAll(bundlePath2)).To(Succeed())
		})

		It("does not allow traffic between containers", func() {
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
			outputs := helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)
			hostIp := outputs.Properties.ContainerIP
			Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			hostPort := findExternalPort(outputs.Properties.MappedPorts, containerPort)

			pid := helpers.GetContainerState(containerId).Pid
			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

			_, _, err := helpers.ExecInContainer(containerId, []string{"c:\\server.exe", containerPort}, true)
			Expect(err).NotTo(HaveOccurred())

			helpers.RunContainer(bundleSpec2, bundlePath2, containerId2)
			helpers.NetworkUp(containerId2, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

			pid = helpers.GetContainerState(containerId2).Pid
			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)

			stdOut, _, err := helpers.ExecInContainer(containerId2, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", hostIp, "--port", strconv.Itoa(hostPort)}, false)
			Expect(err).To(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("An attempt was made to access a socket in a way forbidden by its access permissions"))
		})

		It("can route traffic to the remaining container after the other is deleted", func() {
			helpers.RunContainer(bundleSpec, bundlePath, containerId)
			outputs := helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)

			mappedPorts := []netrules.PortMapping{}
			Expect(json.Unmarshal([]byte(outputs.Properties.MappedPorts), &mappedPorts)).To(Succeed())

			Expect(len(mappedPorts)).To(Equal(1))
			hostPort1 := mappedPorts[0].HostPort

			pid := helpers.GetContainerState(containerId).Pid
			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

			_, _, err := helpers.ExecInContainer(containerId, []string{"c:\\server.exe", containerPort}, true)
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.Get(fmt.Sprintf("http://%s:%d", hostIP, hostPort1))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			data, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %s", containerPort)))

			helpers.RunContainer(bundleSpec2, bundlePath2, containerId2)
			outputs = helpers.NetworkUp(containerId2, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)

			Expect(json.Unmarshal([]byte(outputs.Properties.MappedPorts), &mappedPorts)).To(Succeed())

			Expect(len(mappedPorts)).To(Equal(1))
			hostPort2 := mappedPorts[0].HostPort
			Expect(hostPort2).NotTo(Equal(hostPort1))

			pid = helpers.GetContainerState(containerId2).Pid
			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

			_, _, err = helpers.ExecInContainer(containerId2, []string{"c:\\server.exe", containerPort}, true)
			Expect(err).NotTo(HaveOccurred())

			helpers.DeleteContainer(containerId)
			helpers.NetworkDown(containerId, networkConfigFile)
			helpers.DeleteVolume(containerId)

			resp, err = client.Get(fmt.Sprintf("http://%s:%d", hostIP, hostPort2))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			data, err = ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %s", containerPort)))
		})

		Context("the max outgoing bandwidth is set in the config file", func() {
			var (
				serverURL         string
				clientNetOutRules []byte
			)
			const (
				tinyBandwidth  = 1024 * 1024
				giantBandwidth = 10 * 1024 * 1024
				fileSize       = 10 * 1024 * 1024
			)
			BeforeEach(func() {
				var err error

				helpers.RunContainer(bundleSpec, bundlePath, containerId)
				outputs := helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)
				hostIp, err := localip.LocalIP()
				Expect(err).NotTo(HaveOccurred())
				Expect(helpers.ContainerExists(containerId)).To(BeTrue())
				hostPort := findExternalPort(outputs.Properties.MappedPorts, containerPort)
				serverURL = fmt.Sprintf("http://%s:%d", hostIp, hostPort)

				pid := helpers.GetContainerState(containerId).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

				_, _, err = helpers.ExecInContainer(containerId, []string{"c:\\server.exe", containerPort}, true)
				Expect(err).NotTo(HaveOccurred())

				netOutRule := netrules.NetOut{
					Protocol: netrules.ProtocolAll,
					Networks: []netrules.IPRange{
						{Start: net.ParseIP(hostIp), End: net.ParseIP(hostIp)},
					},
					Ports: []netrules.PortRange{{Start: uint16(hostPort), End: uint16(hostPort)}},
				}
				clientNetOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
				Expect(err).NotTo(HaveOccurred())

				resp, err := http.Get(serverURL)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()
				data, err := ioutil.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %s", containerPort)))
			})

			It("applies the bandwidth limit on the container to outgoing traffic", func() {
				helpers.RunContainer(bundleSpec2, bundlePath2, containerId2)

				pid := helpers.GetContainerState(containerId2).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "client.exe"), clientBin)

				networkConfig.MaximumOutgoingBandwidth = tinyBandwidth
				helpers.WriteNetworkConfig(networkConfig, networkConfigFile)
				helpers.NetworkUp(containerId2, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(clientNetOutRules)), networkConfigFile)

				tinyTime := uploadFile(containerId2, fileSize, serverURL)

				helpers.NetworkDown(containerId2, networkConfigFile)

				networkConfig.MaximumOutgoingBandwidth = giantBandwidth
				helpers.WriteNetworkConfig(networkConfig, networkConfigFile)
				helpers.NetworkUp(containerId2, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(clientNetOutRules)), networkConfigFile)

				giantTime := uploadFile(containerId2, fileSize, serverURL)

				Expect(tinyTime).To(BeNumerically(">", giantTime*7))
			})
		})

		FContext("when the containers share a network namespace", func() {
			BeforeEach(func() {
				bundleSpec2.Windows.Network = &specs.WindowsNetwork{NetworkSharedContainerName: containerId}
				containerPort = "23456"
			})

			FIt("allows traffic between the containers", func() {
				helpers.RunContainer(bundleSpec, bundlePath, containerId)
				helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)
				mp, err := hcsshim.GetLayerMountPath(hcsshim.DriverInfo{
					HomeDir: filepath.Join(os.Getenv("GROOT_IMAGE_STORE"), "volumes"),
					Flavour: 1,
				}, containerId)
				Expect(err).NotTo(HaveOccurred())
				helpers.CopyFile(filepath.Join(mp, "server.exe"), serverBin)

				_, _, err = helpers.ExecInContainer(containerId, []string{"c:\\server.exe", containerPort}, true)
				Expect(err).NotTo(HaveOccurred())

				helpers.RunContainer(bundleSpec2, bundlePath2, containerId2)
				helpers.NetworkUp(containerId2, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

				mp2, err := hcsshim.GetLayerMountPath(hcsshim.DriverInfo{
					HomeDir: filepath.Join(os.Getenv("GROOT_IMAGE_STORE"), "volumes"),
					Flavour: 1,
				}, containerId2)
				Expect(err).NotTo(HaveOccurred())
				helpers.CopyFile(filepath.Join(mp2, "netout.exe"), netoutBin)

				stdOut, _, err := helpers.ExecInContainer(containerId2, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "127.0.0.1", "--port", containerPort}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(stdOut.String()).To(Equal(fmt.Sprintf("connected to 127.0.0.1:%s over tcp", containerPort)))
			})

			Context("when deleting the first container", func() {
				BeforeEach(func() {
					helpers.RunContainer(bundleSpec, bundlePath, containerId)
					helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} }`, networkConfigFile)
					helpers.RunContainer(bundleSpec2, bundlePath2, containerId2)
				})
				It("deletes the main container and the pea container", func() {
					Expect(helpers.ContainerExists(containerId2)).To(BeTrue())
					helpers.DeleteContainer(containerId)
					Expect(helpers.ContainerExists(containerId)).To(BeFalse())
					Expect(helpers.ContainerExists(containerId2)).To(BeFalse())
				})
				It("does not delete the bundle directories", func() {
					helpers.DeleteContainer(containerId)
					Expect(bundlePath).To(BeADirectory())
					Expect(bundlePath2).To(BeADirectory())
				})
				It("unmounts sandbox.vhdx", func() {
					pid := helpers.GetContainerState(containerId2).Pid
					helpers.DeleteContainer(containerId)
					rootPath := filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root")
					Expect(rootPath).NotTo(BeADirectory())

					// if not cleanly unmounted, the mount point is left as a symlink
					_, err := os.Lstat(rootPath)
					Expect(err).NotTo(BeNil())
				})
			})
		})
	})
})
