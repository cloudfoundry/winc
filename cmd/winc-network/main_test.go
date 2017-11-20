package main_test

import (
	"encoding/json"
	"fmt"
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
	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/network"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	subnetRange       string
	gatewayAddress    string
	containerId       string
	tempDir           string
	networkConfigFile string
	networkConfig     network.Config
)

var _ = Describe("networking", func() {
	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "winc-network.config")
		Expect(err).NotTo(HaveOccurred())
		networkConfigFile = filepath.Join(tempDir, "winc-network.json")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	Describe("Create", func() {
		BeforeEach(func() {
			subnetRange, gatewayAddress = randomSubnetAddress()
			networkConfig = network.Config{
				SubnetRange:    subnetRange,
				GatewayAddress: gatewayAddress,
				NetworkName:    gatewayAddress,
			}
		})

		AfterEach(func() {
			deleteNetwork()
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
				networkConfig.MTU = 1234
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
			subnetRange, gatewayAddress = randomSubnetAddress()
			networkConfig = network.Config{
				SubnetRange:    subnetRange,
				GatewayAddress: gatewayAddress,
				NetworkName:    gatewayAddress,
			}
			createNetwork(networkConfig)
		})

		It("deletes the NAT network", func() {
			deleteNetwork()
			psCommand := fmt.Sprintf(`(Get-NetAdapter -name "vEthernet (%s)").InterfaceAlias`, networkConfig.NetworkName)
			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			expectedOutput := fmt.Sprintf("Get-NetAdapter : No MSFT_NetAdapter objects found with property 'Name' equal to 'vEthernet (%s)'", networkConfig.NetworkName)
			Expect(strings.Replace(string(output), "\r\n", "", -1)).To(ContainSubstring(expectedOutput))
		})

		It("deletes the associated firewall rules", func() {
			deleteNetwork()
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
				createContainer()
				subnetRange, gatewayAddress = randomSubnetAddress()
				networkConfig = network.Config{
					SubnetRange:    subnetRange,
					GatewayAddress: gatewayAddress,
					NetworkName:    gatewayAddress,
				}
				createNetwork(networkConfig)
			})

			AfterEach(func() {
				deleteContainerAndNetwork()
			})

			It("sets the host MTU in the container", func() {
				networkUp(`{"Pid": 123, "Properties": {} ,"netin": []}`)

				powershellCommand := fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, gatewayAddress)
				cmd := exec.Command("powershell.exe", "-Command", powershellCommand)
				output, err := cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				hostMTU := strings.TrimSpace(string(output))

				cmd = exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", "(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet *').NlMtu")
				output, err = cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				Expect(strings.TrimSpace(string(output))).To(Equal(hostMTU))
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

					pid := getContainerState(containerId).Pid
					Expect(copyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)).To(Succeed())

					cmd := exec.Command(wincBin, "exec", "-d", containerId, "c:\\server.exe", strconv.Itoa(int(containerPort1)))
					Expect(cmd.Run()).To(Succeed())

					cmd = exec.Command(wincBin, "exec", "-d", containerId, "c:\\server.exe", strconv.Itoa(int(containerPort2)))
					Expect(cmd.Run()).To(Succeed())
				})

				It("generates the correct port mappings and binds them to the container", func() {
					outputs := networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %d},{"host_port": %d, "container_port": %d}]}`, hostPort1, containerPort1, hostPort2, containerPort2))

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

				It("cannot hit a port on the container directly", func() {
					networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %d},{"host_port": %d, "container_port": %d}]}`, hostPort1, containerPort1, hostPort2, containerPort2))

					_, err := client.Get(fmt.Sprintf("http://%s:%d", getContainerIp(containerId), containerPort1))
					Expect(err).To(HaveOccurred())
					errorMsg := "connectex: An attempt was made to access a socket in a way forbidden by its access permissions"
					Expect(err.Error()).To(ContainSubstring(errorMsg))

					_, err = client.Get(fmt.Sprintf("http://%s:%d", getContainerIp(containerId), containerPort2))
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(errorMsg))
				})

				It("creates the correct urlacl in the container", func() {
					networkUp(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)

					output, err := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "netsh http show urlacl url=http://*:8080/ | findstr User").CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), string(output))
					Expect(string(output)).To(ContainSubstring("BUILTIN\\Users"))
				})

				Context("stdin does not contain a port mapping request", func() {
					It("cannot listen on any ports", func() {
						networkUp(`{"Pid": 123, "Properties": {} }`)

						_, err := client.Get(fmt.Sprintf("http://%s:%d", getContainerIp(containerId), containerPort1))
						Expect(err).To(HaveOccurred())
						errorMsg := "connectex: An attempt was made to access a socket in a way forbidden by its access permissions"
						Expect(err.Error()).To(ContainSubstring(errorMsg))

						_, err = client.Get(fmt.Sprintf("http://%s:%d", getContainerIp(containerId), containerPort2))
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring(errorMsg))
					})

					It("prints an empty list of mapped ports", func() {
						outputs := networkUp(`{"Pid": 123, "Properties": {} }`)

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
					pid := getContainerState(containerId).Pid
					Expect(copyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)).To(Succeed())
				})

				It("cannot resolve DNS", func() {
					networkUp(`{"Pid": 123, "Properties": {}}`)

					cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "dns", "--addr", "www.google.com")
					output, err := cmd.CombinedOutput()
					Expect(err).To(HaveOccurred(), string(output))
					Expect(string(output)).To(ContainSubstring("lookup www.google.com: no such host"))
				})

				It("cannot connect to a remote host over TCP", func() {
					networkUp(`{"Pid": 123, "Properties": {}}`)

					cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53")
					output, err := cmd.CombinedOutput()
					Expect(err).To(HaveOccurred(), string(output))
					errStr := "dial tcp 8.8.8.8:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
					Expect(strings.TrimSpace(string(output))).To(Equal(errStr))
				})

				It("cannot connect to a remote host over UDP", func() {
					networkUp(`{"Pid": 123, "Properties": {}}`)

					cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53")
					output, err := cmd.CombinedOutput()
					Expect(err).To(HaveOccurred(), string(output))
					Expect(string(output)).To(ContainSubstring("failed to exchange: read udp"))
					Expect(string(output)).To(ContainSubstring("8.8.8.8:53: i/o timeout"))
				})

				It("cannot connect to a remote host over ICMP", func() {
					networkUp(`{"Pid": 123, "Properties": {}}`)

					cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8")
					output, err := cmd.CombinedOutput()
					Expect(err).To(HaveOccurred(), string(output))
					Expect(string(output)).To(ContainSubstring("Ping statistics for 8.8.8.8"))
					Expect(string(output)).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
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

					pid := getContainerState(containerId).Pid
					Expect(copyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)).To(Succeed())
				})

				Context("netout allows udp", func() {
					BeforeEach(func() {
						var err error

						netOutRule.Protocol = netrules.ProtocolUDP
						netOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
						Expect(err).NotTo(HaveOccurred())
					})

					It("can connect to a remote host over UDP", func() {
						networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53")
						output, err := cmd.CombinedOutput()
						Expect(err).NotTo(HaveOccurred(), string(output))
						Expect(string(output)).To(ContainSubstring("recieved response to DNS query from 8.8.8.8:53 over UDP"))
					})

					It("cannot connect to a remote host over UDP prohibited by netout", func() {
						networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.4.4", "--port", "53")
						output, err := cmd.CombinedOutput()
						Expect(err).To(HaveOccurred(), string(output))
						Expect(string(output)).To(ContainSubstring("failed to exchange: read udp"))
						Expect(string(output)).To(ContainSubstring("8.8.4.4:53: i/o timeout"))
					})

					Context("netout allows udp on port 53", func() {
						BeforeEach(func() {
							var err error

							netOutRule.Networks = []netrules.IPRange{
								{Start: net.ParseIP("0.0.0.0"), End: net.ParseIP("255.255.255.255")},
							}

							netOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
							Expect(err).NotTo(HaveOccurred())
						})

						It("can resolve DNS", func() {
							networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

							cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "dns", "--addr", "www.google.com")
							output, err := cmd.CombinedOutput()
							Expect(err).NotTo(HaveOccurred(), string(output))
							Expect(string(output)).To(ContainSubstring("found addr"))
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
						networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53")
						output, err := cmd.CombinedOutput()
						Expect(err).NotTo(HaveOccurred(), string(output))
						Expect(strings.TrimSpace(string(output))).To(Equal("connected to 8.8.8.8:53 over tcp"))
					})

					It("cannot connect to a remote server over TCP prohibited by netout", func() {
						networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						cmd := exec.Command(wincBin, "exec", "-u", "vcap", containerId, "c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.4.4", "--port", "53")
						output, err := cmd.CombinedOutput()
						Expect(err).To(HaveOccurred(), string(output))
						errStr := "dial tcp 8.8.4.4:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
						Expect(strings.TrimSpace(string(output))).To(Equal(errStr))
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
						networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8")
						output, err := cmd.CombinedOutput()
						Expect(err).NotTo(HaveOccurred(), string(output))
						Expect(string(output)).To(ContainSubstring("Ping statistics for 8.8.8.8"))
						Expect(string(output)).To(ContainSubstring("Packets: Sent = 4, Received = 4, Lost = 0 (0% loss)"))
					})

					It("cannot connect to a remote host over ICMP prohibited by netout", func() {
						networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.4.4")
						output, err := cmd.CombinedOutput()
						Expect(err).To(HaveOccurred(), string(output))
						Expect(string(output)).To(ContainSubstring("Ping statistics for 8.8.4.4"))
						Expect(string(output)).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
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
						networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.8.8", "--port", "53")
						output, err := cmd.CombinedOutput()
						Expect(err).NotTo(HaveOccurred(), string(output))
						Expect(string(output)).To(ContainSubstring("recieved response to DNS query from 8.8.8.8:53 over UDP"))

						cmd = exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53")
						output, err = cmd.CombinedOutput()
						Expect(err).NotTo(HaveOccurred(), string(output))
						Expect(strings.TrimSpace(string(output))).To(Equal("connected to 8.8.8.8:53 over tcp"))

						cmd = exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8")
						output, err = cmd.CombinedOutput()
						Expect(err).NotTo(HaveOccurred(), string(output))
						Expect(string(output)).To(ContainSubstring("Ping statistics for 8.8.8.8"))
						Expect(string(output)).To(ContainSubstring("Packets: Sent = 4, Received = 4, Lost = 0 (0% loss)"))
					})

					It("blocks access over all protocols to prohibited remote hosts", func() {
						networkUp(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)))

						cmd := exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.4.4", "--port", "53")
						output, err := cmd.CombinedOutput()
						Expect(err).To(HaveOccurred(), string(output))
						Expect(string(output)).To(ContainSubstring("failed to exchange: read udp"))
						Expect(string(output)).To(ContainSubstring("8.8.4.4:53: i/o timeout"))

						cmd = exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.4.4", "--port", "53")
						output, err = cmd.CombinedOutput()
						Expect(err).To(HaveOccurred(), string(output))
						errStr := "dial tcp 8.8.4.4:53: connectex: An attempt was made to access a socket in a way forbidden by its access permissions."
						Expect(strings.TrimSpace(string(output))).To(Equal(errStr))

						cmd = exec.Command(wincBin, "exec", containerId, "c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.4.4")
						output, err = cmd.CombinedOutput()
						Expect(err).To(HaveOccurred(), string(output))
						Expect(string(output)).To(ContainSubstring("Ping statistics for 8.8.4.4"))
						Expect(string(output)).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
					})
				})

			})
		})

		Context("custom MTU", func() {
			BeforeEach(func() {
				createContainer()
				subnetRange, gatewayAddress = randomSubnetAddress()
				networkConfig = network.Config{
					SubnetRange:    subnetRange,
					GatewayAddress: gatewayAddress,
					NetworkName:    gatewayAddress,
					MTU:            1405,
				}
				createNetwork(networkConfig)
			})

			AfterEach(func() {
				deleteContainerAndNetwork()
			})

			It("sets the network MTU on the internal container NIC", func() {
				networkUp(`{"Pid": 123, "Properties": {} ,"netin": []}`)

				cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", `(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias "vEthernet*").NlMtu`)
				output, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
				Expect(strings.TrimSpace(string(output))).To(Equal("1405"))
			})
		})

		Context("custom DNS Servers", func() {
			BeforeEach(func() {
				createContainer()
				subnetRange, gatewayAddress = randomSubnetAddress()
				networkConfig = network.Config{
					SubnetRange:    subnetRange,
					GatewayAddress: gatewayAddress,
					NetworkName:    gatewayAddress,
					DNSServers:     []string{"1.1.1.1", "2.2.2.2"},
				}
				createNetwork(networkConfig)
			})

			AfterEach(func() {
				deleteContainerAndNetwork()
			})

			It("uses those IP addresses as DNS servers", func() {
				networkUp(`{"Pid": 123, "Properties": {} ,"netin": []}`)

				cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", `(Get-DnsClientServerAddress -InterfaceAlias 'vEthernet*' -AddressFamily IPv4).ServerAddresses -join ","`)
				output, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
				Expect(strings.TrimSpace(string(output))).To(Equal("1.1.1.1,2.2.2.2"))
			})
		})
	})

	Describe("Down", func() {
		BeforeEach(func() {
			createContainer()
			subnetRange, gatewayAddress = randomSubnetAddress()
			networkConfig = network.Config{
				SubnetRange:    subnetRange,
				GatewayAddress: gatewayAddress,
				NetworkName:    gatewayAddress,
			}
			createNetwork(networkConfig)

			output, err := exec.Command(wincNetworkBin, "--action", "create", "--configFile", networkConfigFile).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))

			networkUp(`{"Pid": 123, "Properties": {}}`)
			Expect(len(allEndpoints(containerId))).To(Equal(1))
		})

		AfterEach(func() {
			deleteContainerAndNetwork()
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

			subnetRange, gatewayAddress = randomSubnetAddress()
			networkConfig = network.Config{
				SubnetRange:    subnetRange,
				GatewayAddress: gatewayAddress,
				NetworkName:    gatewayAddress,
			}
		})

		AfterEach(func() {
			deleteNetwork()
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

func networkUp(input string) network.UpOutputs {
	cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
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

func deleteNetwork() {
	output, err := exec.Command(wincNetworkBin, "--action", "delete", "--configFile", networkConfigFile).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func createContainer() {
	containerId = filepath.Base(bundlePath)
	bundleSpec := runtimeSpecGenerator(createSandbox("C:\\run\\winc", rootfsPath, containerId), containerId)
	containerConfig, err := json.Marshal(&bundleSpec)
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), containerConfig, 0666)).To(Succeed())

	output, err := exec.Command(wincBin, "create", "-b", bundlePath, containerId).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func deleteContainerAndNetwork() {
	output, err := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))

	if containerExists(containerId) {
		output, err = exec.Command(wincBin, "delete", containerId).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
	}

	output, err = exec.Command(wincImageBin, "--store", "C:\\run\\winc", "delete", containerId).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))

	deleteNetwork()
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

func randomSubnetAddress() (string, string) {
	for {
		subnet, gateway := randomValidSubnetAddress()
		_, err := hcsshim.GetHNSNetworkByName(subnet)
		if err != nil {
			Expect(err).To(MatchError(ContainSubstring("Network " + subnet + " not found")))
			return subnet, gateway
		}
	}
}

func randomValidSubnetAddress() (string, string) {
	randomOctet := rand.Intn(256)
	gatewayAddress := fmt.Sprintf("172.16.%d.1", randomOctet)
	subnet := fmt.Sprintf("172.16.%d.0/30", randomOctet)
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
