package main_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/localip"
	helpers "code.cloudfoundry.org/winc/cmd/helpers"
	"code.cloudfoundry.org/winc/filelock"
	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/network"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	containerId       string
	bundlePath        string
	tempDir           string
	networkConfigFile string
	networkConfig     network.Config
)

const gatewayFileName = "c:\\var\\vcap\\data\\winc-network\\gateways.json"

var _ = Describe("networking", func() {
	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "winc-network.config")
		Expect(err).NotTo(HaveOccurred())
		networkConfigFile = filepath.Join(tempDir, "winc-network.json")

		bundlePath, err = ioutil.TempDir("", "win-container-1")
		Expect(err).NotTo(HaveOccurred())
		containerId = filepath.Base(bundlePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempDir)).To(Succeed())
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Describe("Create", func() {
		BeforeEach(func() {
			networkConfig = generateNetworkConfig()
		})

		AfterEach(func() {
			deleteNetwork(networkConfig)
		})

		It("creates the network with the correct name", func() {
			createNetwork(networkConfig)

			psCommand := fmt.Sprintf(`(Get-NetAdapter -name "vEthernet (%s)").InterfaceAlias`, networkConfig.NetworkName)
			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			Expect(strings.TrimSpace(string(output))).To(Equal(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName)))
		})

		It("creates the network with the correct subnet range", func() {
			createNetwork(networkConfig)

			psCommand := fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.IPAddress`, networkConfig.NetworkName)
			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			ipAddress := strings.TrimSuffix(strings.TrimSpace(string(output)), "1") + "0"

			psCommand = fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.PrefixLength`, networkConfig.NetworkName)
			output, err = exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			prefixLength := strings.TrimSpace(string(output))

			Expect(fmt.Sprintf("%s/%s", ipAddress, prefixLength)).To(Equal(networkConfig.SubnetRange))
		})

		It("creates the network with the correct gateway address", func() {
			createNetwork(networkConfig)

			psCommand := fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.IPAddress`, networkConfig.NetworkName)
			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			Expect(strings.TrimSpace(string(output))).To(Equal(networkConfig.GatewayAddress))
		})

		It("creates the network with mtu matching that of the host", func() {
			psCommand := `(Get-NetAdapter -Physical).Name`
			output, err := exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			physicalNetworkName := strings.TrimSpace(string(output))

			psCommand = fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias '%s').NlMtu`, physicalNetworkName)
			output, err = exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			physicalMTU := strings.TrimSpace(string(output))

			createNetwork(networkConfig)

			psCommand = fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, networkConfig.NetworkName)
			output, err = exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			virtualMTU := strings.TrimSpace(string(output))

			Expect(virtualMTU).To(Equal(physicalMTU))
		})

		Context("mtu is set in the config", func() {
			BeforeEach(func() {
				networkConfig.MTU = 1400
			})

			It("creates the network with the configured mtu", func() {
				createNetwork(networkConfig)

				psCommand := fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, networkConfig.NetworkName)
				output, err := exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				virtualMTU := strings.TrimSpace(string(output))

				Expect(virtualMTU).To(Equal(strconv.Itoa(networkConfig.MTU)))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			networkConfig = generateNetworkConfig()
			createNetwork(networkConfig)
		})

		It("deletes the NAT network", func() {
			deleteNetwork(networkConfig)
			psCommand := fmt.Sprintf(`(Get-NetAdapter -name "vEthernet (%s)").InterfaceAlias`, networkConfig.NetworkName)
			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			expectedOutput := fmt.Sprintf("Get-NetAdapter : No MSFT_NetAdapter objects found with property 'Name' equal to 'vEthernet (%s)'", networkConfig.NetworkName)
			Expect(strings.Replace(string(output), "\r\n", "", -1)).To(ContainSubstring(expectedOutput))
		})

		It("deletes the associated firewall rules", func() {
			deleteNetwork(networkConfig)
			getFirewallRule := fmt.Sprintf(`Get-NetFirewallRule -DisplayName "%s"`, networkConfig.NetworkName)
			output, err := exec.Command("powershell.exe", "-Command", getFirewallRule).CombinedOutput()
			Expect(err).To(HaveOccurred())
			expectedOutput := fmt.Sprintf(`Get-NetFirewallRule : No MSFT_NetFirewallRule objects found with property 'DisplayName' equal to '%s'`, networkConfig.NetworkName)
			Expect(strings.Replace(string(output), "\r\n", "", -1)).To(ContainSubstring(expectedOutput))
		})
	})

	Describe("Up", func() {
		Context("default network config", func() {
			BeforeEach(func() {
				createContainer(containerId)
				networkConfig = generateNetworkConfig()
				createNetwork(networkConfig)
			})

			AfterEach(func() {
				deleteContainerAndNetwork(containerId, networkConfig)
			})

			It("sets the host MTU in the container", func() {
				networkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`)

				powershellCommand := fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, networkConfig.GatewayAddress)
				cmd := exec.Command("powershell.exe", "-Command", powershellCommand)
				output, err := cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				hostMTU := strings.TrimSpace(string(output))

				stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"powershell.exe", "-Command", "(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet *').NlMtu"}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(stdout.String())).To(Equal(hostMTU))
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

					pid := helpers.GetContainerState(wincBin, containerId).Pid
					Expect(helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)).To(Succeed())

					_, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\server.exe", strconv.Itoa(int(containerPort1))}, true)
					Expect(err).NotTo(HaveOccurred())
					_, _, err = helpers.ExecInContainer(wincBin, containerId, []string{"c:\\server.exe", strconv.Itoa(int(containerPort2))}, true)
					Expect(err).NotTo(HaveOccurred())
				})

				It("generates the correct port mappings and binds them to the container", func() {
					outputs := networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %d},{"host_port": %d, "container_port": %d}]}`, hostPort1, containerPort1, hostPort2, containerPort2))

					mappedPorts := []netrules.PortMapping{}
					Expect(json.Unmarshal([]byte(outputs.Properties.MappedPorts), &mappedPorts)).To(Succeed())

					Expect(len(mappedPorts)).To(Equal(2))

					Expect(mappedPorts[0].ContainerPort).To(Equal(containerPort1))
					Expect(mappedPorts[0].HostPort).NotTo(Equal(hostPort1))

					Expect(mappedPorts[1].ContainerPort).To(Equal(containerPort2))
					Expect(mappedPorts[1].HostPort).To(Equal(hostPort2))

					hostPort1 = mappedPorts[0].HostPort

					hostIp := outputs.Properties.ContainerIP

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
					networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %d},{"host_port": %d, "container_port": %d}]}`, hostPort1, containerPort1, hostPort2, containerPort2))

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
					networkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)

					stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"cmd.exe", "/C", "netsh http show urlacl url=http://*:8080/ | findstr User"}, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(stdout.String()).To(ContainSubstring("BUILTIN\\Users"))
				})

				Context("stdin does not contain a port mapping request", func() {
					It("cannot listen on any ports", func() {
						networkUp(containerId, `{"Pid": 123, "Properties": {} }`)

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
						outputs := networkUp(containerId, `{"Pid": 123, "Properties": {} }`)

						Expect(outputs.Properties.MappedPorts).To(Equal("[]"))
						Expect(outputs.Properties.DeprecatedHostIP).To(Equal("255.255.255.255"))

						localIP, err := localip.LocalIP()
						Expect(err).NotTo(HaveOccurred())
						Expect(outputs.Properties.ContainerIP).To(Equal(localIP))
					})
				})
			})

			Context("stdin does not contain net out rules", func() {
				BeforeEach(func() {
					pid := helpers.GetContainerState(wincBin, containerId).Pid
					Expect(helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)).To(Succeed())
				})

				It("cannot resolve DNS", func() {
					networkUp(containerId, `{"Pid": 123, "Properties": {}}`)

					stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "dns", "--addr", "www.google.com"}, false)
					Expect(err).To(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("lookup www.google.com: no such host"))
				})

				It("cannot connect to a remote host over TCP", func() {
					networkUp(containerId, `{"Pid": 123, "Properties": {}}`)

					stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())

					errStr := "dial tcp 8.8.8.8:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
					Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))
				})

				It("cannot connect to a remote host over UDP", func() {
					networkUp(containerId, `{"Pid": 123, "Properties": {}}`)

					stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53"}, false)
					Expect(err).To(HaveOccurred())

					Expect(stdout.String()).To(ContainSubstring("failed to exchange: read udp"))
					Expect(stdout.String()).To(ContainSubstring("8.8.8.8:53: i/o timeout"))
				})

				It("cannot connect to a remote host over ICMP", func() {
					Skip("ping.exe elevates to admin, breaking this test")
					networkUp(containerId, `{"Pid": 123, "Properties": {}}`)

					stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
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

					pid := helpers.GetContainerState(wincBin, containerId).Pid
					Expect(helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)).To(Succeed())
				})

				Context("netout allows udp", func() {
					BeforeEach(func() {
						var err error

						netOutRule.Protocol = netrules.ProtocolUDP
						netOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
						Expect(err).NotTo(HaveOccurred())
					})

					It("can connect to a remote host over UDP", func() {
						networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53"}, false)
						Expect(err).NotTo(HaveOccurred())

						Expect(stdout.String()).To(ContainSubstring("recieved response to DNS query from 8.8.8.8:53 over UDP"))
					})

					It("cannot connect to a remote host over UDP prohibited by netout", func() {
						networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.4.4", "--port", "53"}, false)
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
							networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

							stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "dns", "--addr", "www.google.com"}, false)
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
						networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
						Expect(err).NotTo(HaveOccurred())

						Expect(strings.TrimSpace(stdout.String())).To(Equal("connected to 8.8.8.8:53 over tcp"))
					})

					It("cannot connect to a remote server over TCP prohibited by netout", func() {
						networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.4.4", "--port", "53"}, false)
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
						networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
						Expect(err).NotTo(HaveOccurred())

						Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.8.8"))
						Expect(stdout.String()).NotTo(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
						Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received ="))
					})

					It("cannot connect to a remote host over ICMP prohibited by netout", func() {
						Skip("ping.exe elevates to admin, breaking this test")
						networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.4.4"}, false)
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
						networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53"}, false)
						Expect(err).NotTo(HaveOccurred())
						Expect(stdout.String()).To(ContainSubstring("recieved response to DNS query from 8.8.8.8:53 over UDP"))

						stdout, _, err = helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
						Expect(err).NotTo(HaveOccurred())
						Expect(strings.TrimSpace(stdout.String())).To(Equal("connected to 8.8.8.8:53 over tcp"))

						stdout, _, err = helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
						Expect(err).NotTo(HaveOccurred())
						Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.8.8"))
						Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 4, Lost = 0 (0% loss)"))
					})

					It("blocks access over all protocols to prohibited remote hosts", func() {
						networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.4.4", "--port", "53"}, false)
						Expect(err).To(HaveOccurred())
						Expect(stdout.String()).To(ContainSubstring("failed to exchange: read udp"))
						Expect(stdout.String()).To(ContainSubstring("8.8.4.4:53: i/o timeout"))

						stdout, _, err = helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.4.4", "--port", "53"}, false)
						Expect(err).To(HaveOccurred())
						errStr := "dial tcp 8.8.4.4:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
						Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))

						// ping.exe elevates to admin, breaking this test

						//	stdout, _, err = helpers.ExecInContainer(wincBin, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.4.4"}, false)
						//	Expect(err).To(HaveOccurred())
						//	Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.4.4"))
						//	Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
					})
				})

			})
		})

		Context("custom MTU", func() {
			BeforeEach(func() {
				createContainer(containerId)
				networkConfig = generateNetworkConfig()
				networkConfig.MTU = 1405
				createNetwork(networkConfig)
			})

			AfterEach(func() {
				deleteContainerAndNetwork(containerId, networkConfig)
			})

			It("sets the network MTU on the internal container NIC", func() {
				networkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`)

				stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"powershell.exe", "-Command", `(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias "vEthernet*").NlMtu`}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(stdout.String())).To(Equal("1405"))
			})
		})

		Context("custom DNS Servers", func() {
			BeforeEach(func() {
				createContainer(containerId)
				networkConfig = generateNetworkConfig()
				networkConfig.DNSServers = []string{"8.8.8.8", "8.8.4.4"}
				createNetwork(networkConfig)
			})

			AfterEach(func() {
				deleteContainerAndNetwork(containerId, networkConfig)
			})

			It("uses those IP addresses as DNS servers", func() {
				networkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`)

				stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"powershell.exe", "-Command", `(Get-DnsClientServerAddress -InterfaceAlias 'vEthernet*' -AddressFamily IPv4).ServerAddresses -join ","`}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(stdout.String())).To(Equal("8.8.8.8,8.8.4.4"))
			})

			It("allows traffic to those servers", func() {
				networkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`)

				pid := helpers.GetContainerState(wincBin, containerId).Pid
				Expect(helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)).To(Succeed())

				stdout, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(stdout.String())).To(Equal("connected to 8.8.8.8:53 over tcp"))
			})
		})
	})

	Describe("Down", func() {
		BeforeEach(func() {
			createContainer(containerId)
			networkConfig = generateNetworkConfig()
			createNetwork(networkConfig)

			output, err := exec.Command(wincNetworkBin, "--action", "create", "--configFile", networkConfigFile).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))

			networkUp(containerId, `{"Pid": 123, "Properties": {}}`)
			Expect(len(allEndpoints(containerId))).To(Equal(1))
		})

		AfterEach(func() {
			deleteContainerAndNetwork(containerId, networkConfig)
		})

		It("deletes the endpoint", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			Expect(len(allEndpoints(containerId))).To(Equal(0))
			Expect(endpointExists(containerId)).To(BeFalse())
		})

		Context("when the endpoint does not exist", func() {
			It("does nothing", func() {
				cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", "some-nonexistant-id")
				output, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
			})
		})

		Context("when the container is deleted before the endpoint", func() {
			BeforeEach(func() {
				output, err := exec.Command(wincBin, "delete", containerId).CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
			})

			It("deletes the endpoint", func() {
				Expect(endpointExists(containerId)).To(BeTrue())
				cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId)
				output, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
				Expect(endpointExists(containerId)).To(BeFalse())
			})
		})
	})

	Context("two containers are running", func() {
		var (
			bundlePath2   string
			containerId2  string
			containerPort string
		)

		BeforeEach(func() {
			var err error
			bundlePath2, err = ioutil.TempDir("", "win-container-2")
			Expect(err).NotTo(HaveOccurred())
			containerId2 = filepath.Base(bundlePath2)

			containerPort = "12345"

			networkConfig = generateNetworkConfig()
			createNetwork(networkConfig)
		})

		AfterEach(func() {
			endpointDown(containerId2)
			deleteContainer(containerId2)
			deleteImage(containerId2)
			deleteContainerAndNetwork(containerId, networkConfig)
			Expect(os.RemoveAll(bundlePath2)).To(Succeed())
		})

		It("does not allow traffic between containers", func() {
			createContainer(containerId)
			outputs := networkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort))
			containerIp := outputs.Properties.ContainerIP

			pid := helpers.GetContainerState(wincBin, containerId).Pid
			Expect(helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)).To(Succeed())

			_, _, err := helpers.ExecInContainer(wincBin, containerId, []string{"c:\\server.exe", containerPort}, true)
			Expect(err).NotTo(HaveOccurred())

			createContainer(containerId2)
			networkUp(containerId2, `{"Pid": 123, "Properties": {}}`)

			pid = helpers.GetContainerState(wincBin, containerId2).Pid
			Expect(helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)).To(Succeed())

			stdOut, _, err := helpers.ExecInContainer(wincBin, containerId2, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", containerIp, "--port", containerPort}, false)
			Expect(err).To(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("An attempt was made to access a socket in a way forbidden by its access permissions"))
		})
	})

	Context("when provided --log <log-file>", func() {
		var (
			logFile string
			tempDir string
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "log-dir")
			Expect(err).NotTo(HaveOccurred())

			logFile = filepath.Join(tempDir, "winc-network.log")

			networkConfig = generateNetworkConfig()
		})

		AfterEach(func() {
			deleteNetwork(networkConfig)
			Expect(os.RemoveAll(tempDir)).To(Succeed())
		})

		Context("when the provided log file path does not exist", func() {
			BeforeEach(func() {
				logFile = filepath.Join(tempDir, "some-dir", "winc-network.log")
			})

			It("creates the full path", func() {
				createNetwork(networkConfig, "--log", logFile)

				Expect(logFile).To(BeAnExistingFile())
			})
		})

		Context("when it runs successfully", func() {
			It("does not log to the specified file", func() {
				createNetwork(networkConfig, "--log", logFile)

				contents, err := ioutil.ReadFile(logFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(BeEmpty())
			})

			Context("when provided --debug", func() {
				It("outputs debug level logs", func() {
					createNetwork(networkConfig, "--log", logFile, "--debug")

					contents, err := ioutil.ReadFile(logFile)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(contents)).NotTo(BeEmpty())
				})
			})
		})

		Context("when it errors", func() {
			BeforeEach(func() {
				c, err := json.Marshal(networkConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(ioutil.WriteFile(networkConfigFile, c, 0644)).To(Succeed())
			})

			It("logs errors to the specified file", func() {
				exec.Command(wincNetworkBin, "--action", "some-invalid-action", "--log", logFile).CombinedOutput()

				contents, err := ioutil.ReadFile(logFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).NotTo(BeEmpty())
				Expect(string(contents)).To(ContainSubstring("some-invalid-action"))
			})
		})
	})
})

func networkUp(id, input string) network.UpOutputs {
	cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", id)
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))

	var upOutput network.UpOutputs
	Expect(json.Unmarshal(output, &upOutput)).To(Succeed())
	return upOutput
}

func createNetwork(config network.Config, extraArgs ...string) {
	c, err := json.Marshal(config)
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(networkConfigFile, c, 0644)).To(Succeed())

	args := []string{"--action", "create", "--configFile", networkConfigFile}
	args = append(args, extraArgs...)
	output, err := exec.Command(wincNetworkBin, args...).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func deleteNetwork(config network.Config) {
	gatewayFile := filelock.NewLocker(gatewayFileName)
	f, err := gatewayFile.Open()
	defer f.Close()
	Expect(err).NotTo(HaveOccurred())

	oldGatewaysInUse := loadGatewaysInUse(f)
	var newGatewaysInUse []string

	for _, n := range oldGatewaysInUse {
		if n != config.GatewayAddress {
			newGatewaysInUse = append(newGatewaysInUse, n)
		}
	}

	writeGatewaysInUse(f, newGatewaysInUse)

	output, err := exec.Command(wincNetworkBin, "--action", "delete", "--configFile", networkConfigFile).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func deleteImage(id string) {
	output, err := exec.Command(wincImageBin, "--store", "C:\\run\\winc", "delete", id).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func createContainer(id string) {
	bundleSpec := helpers.GenerateRuntimeSpec(helpers.CreateSandbox(wincImageBin, "C:\\run\\winc", rootfsPath, id))
	containerConfig, err := json.Marshal(&bundleSpec)
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(filepath.Join(os.TempDir(), id, "config.json"), containerConfig, 0666)).To(Succeed())

	output, err := exec.Command(wincBin, "create", "-b", filepath.Join(os.TempDir(), id), id).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func deleteContainer(id string) {
	if helpers.ContainerExists(id) {
		output, err := exec.Command(wincBin, "delete", id).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
	}
}

func endpointDown(id string) {
	output, err := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", id).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func deleteContainerAndNetwork(id string, config network.Config) {
	endpointDown(id)
	deleteContainer(id)
	deleteImage(id)
	deleteNetwork(config)
}

func getContainerIp(containerId string) net.IP {
	container, err := hcsshim.OpenContainer(containerId)
	Expect(err).NotTo(HaveOccurred(), "no containers with id: "+containerId)

	stats, err := container.Statistics()
	Expect(err).NotTo(HaveOccurred())

	Expect(stats.Network).NotTo(BeEmpty(), "container has no networks attached: "+containerId)
	endpoint, err := hcsshim.GetHNSEndpointByID(stats.Network[0].EndpointId)
	Expect(err).NotTo(HaveOccurred())

	return endpoint.IPAddress
}

func randomPort() int {
	l, err := net.Listen("tcp", ":0")
	Expect(err).NotTo(HaveOccurred())
	defer l.Close()
	split := strings.Split(l.Addr().String(), ":")
	port, err := strconv.Atoi(split[len(split)-1])
	Expect(err).NotTo(HaveOccurred())
	return port
}

func endpointExists(endpointName string) bool {
	_, err := hcsshim.GetHNSEndpointByName(endpointName)
	if err != nil {
		if err.Error() == fmt.Sprintf("Endpoint %s not found", endpointName) {
			return false
		}

		Expect(err).NotTo(HaveOccurred())
	}

	return true
}

func generateNetworkConfig() network.Config {
	var subnet, gateway string

	gatewayFile := filelock.NewLocker(gatewayFileName)
	f, err := gatewayFile.Open()
	defer f.Close()
	Expect(err).NotTo(HaveOccurred())

	gatewaysInUse := loadGatewaysInUse(f)

	for {
		subnet, gateway = randomValidSubnetAddress()
		if !natNetworkInUse(gateway, gatewaysInUse) && !collideWithHost(gateway) {
			gatewaysInUse = append(gatewaysInUse, gateway)
			break
		}
	}

	writeGatewaysInUse(f, gatewaysInUse)

	return network.Config{
		SubnetRange:    subnet,
		GatewayAddress: gateway,
		NetworkName:    gateway,
	}
}

func loadGatewaysInUse(f filelock.LockedFile) []string {
	data := make([]byte, 10240)
	n, err := f.Read(data)
	if err != nil {
		Expect(err).To(Equal(io.EOF))
		data = []byte("[]")
		n = 2
	}

	gateways := []string{}
	Expect(json.Unmarshal(data[:n], &gateways)).To(Succeed())

	return gateways
}

func writeGatewaysInUse(f filelock.LockedFile, gateways []string) {
	data, err := json.Marshal(gateways)
	Expect(err).NotTo(HaveOccurred())

	_, err = f.Seek(0, io.SeekStart)
	Expect(err).NotTo(HaveOccurred())
	Expect(f.Truncate(0)).To(Succeed())

	_, err = f.Write(data)
	Expect(err).NotTo(HaveOccurred())
}

func natNetworkInUse(name string, inuse []string) bool {
	for _, n := range inuse {
		if name == n {
			return true
		}
	}

	_, err := hcsshim.GetHNSNetworkByName(name)
	if err != nil {
		Expect(err).To(MatchError(ContainSubstring("Network " + name + " not found")))
		return false
	}

	return true
}

func collideWithHost(gateway string) bool {
	hostip, err := localip.LocalIP()
	Expect(err).NotTo(HaveOccurred())

	hostbytes := strings.Split(hostip, ".")
	gatewaybytes := strings.Split(gateway, ".")

	// only need to compare first 3 bytes since mask is /24
	return hostbytes[0] == gatewaybytes[0] &&
		hostbytes[1] == gatewaybytes[1] &&
		hostbytes[2] == gatewaybytes[2]
}

func randomValidSubnetAddress() (string, string) {
	randomOctet := rand.Intn(256)
	gatewayAddress := fmt.Sprintf("172.16.%d.1", randomOctet)
	subnet := fmt.Sprintf("172.16.%d.0/24", randomOctet)
	return subnet, gatewayAddress
}

func ipMask(maskLen int) net.IP {
	mask := net.CIDRMask(maskLen, 32)
	return net.IPv4(
		mask[0],
		mask[1],
		mask[2],
		mask[3],
	)
}

func allEndpoints(containerID string) []string {
	container, err := hcsshim.OpenContainer(containerID)
	Expect(err).To(Succeed())

	stats, err := container.Statistics()
	Expect(err).To(Succeed())

	var endpointIDs []string
	for _, network := range stats.Network {
		endpointIDs = append(endpointIDs, network.EndpointId)
	}

	return endpointIDs
}
