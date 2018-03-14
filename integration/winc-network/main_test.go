package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network"
	"code.cloudfoundry.org/winc/network/netrules"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var (
	containerId string
	bundlePath  string
	tempDir     string
)

const gatewayFileName = "c:\\var\\vcap\\data\\winc-network\\gateways.json"

var _ = Describe("networking", func() {
	BeforeEach(func() {
		var err error
		//		tempDir, err = ioutil.TempDir("", "winc-network.config")
		//		Expect(err).NotTo(HaveOccurred())
		//		networkConfigFile = filepath.Join(tempDir, "winc-network.json")

		bundlePath, err = ioutil.TempDir("", "win-container-1")
		Expect(err).NotTo(HaveOccurred())
		containerId = filepath.Base(bundlePath)
	})

	AfterEach(func() {
		//Expect(os.RemoveAll(tempDir)).To(Succeed())
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	//	Describe("Create", func() {
	//		BeforeEach(func() {
	//			networkConfig = helpers.GenerateNetworkConfig()
	//		})
	//
	//		AfterEach(func() {
	//			helpers.DeleteNetwork(networkConfig, networkConfigFile)
	//			Expect(os.Remove(networkConfigFile)).To(Succeed())
	//		})
	//
	//		It("creates the network with the correct name", func() {
	//			helpers.CreateNetwork(networkConfig, networkConfigFile)
	//
	//			hostIP := "192.168.158.128"
	//			ni := netinterface.NetInterface{}
	//			iface, err := ni.ByIP(hostIP)
	//			Expect(err).NotTo(HaveOccurred())
	//
	//			psCommand := fmt.Sprintf(`(Get-NetAdapter -name "%s").InterfaceAlias`, iface.Name)
	//			//	psCommand := fmt.Sprintf(`(Get-NetAdapter -name "vEthernet (%s)").InterfaceAlias`, networkConfig.NetworkName)
	//			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
	//			Expect(err).NotTo(HaveOccurred(), string(output))
	//			Expect(strings.TrimSpace(string(output))).To(Equal(fmt.Sprintf("vEthernet (%s)", networkConfig.NetworkName)))
	//		})
	//
	//		It("creates the network with the correct subnet range", func() {
	//			helpers.CreateNetwork(networkConfig, networkConfigFile)
	//
	//			psCommand := fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.IPAddress`, networkConfig.NetworkName)
	//			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
	//			Expect(err).NotTo(HaveOccurred(), string(output))
	//			ipAddress := strings.TrimSuffix(strings.TrimSpace(string(output)), "1") + "0"
	//
	//			psCommand = fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.PrefixLength`, networkConfig.NetworkName)
	//			output, err = exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
	//			Expect(err).NotTo(HaveOccurred(), string(output))
	//			prefixLength := strings.TrimSpace(string(output))
	//
	//			Expect(fmt.Sprintf("%s/%s", ipAddress, prefixLength)).To(Equal(networkConfig.SubnetRange))
	//		})
	//
	//		It("creates the network with the correct gateway address", func() {
	//			helpers.CreateNetwork(networkConfig, networkConfigFile)
	//
	//			psCommand := fmt.Sprintf(`(get-netipconfiguration -interfacealias "vEthernet (%s)").IPv4Address.IPAddress`, networkConfig.NetworkName)
	//			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
	//			Expect(err).NotTo(HaveOccurred(), string(output))
	//			Expect(strings.TrimSpace(string(output))).To(Equal(networkConfig.GatewayAddress))
	//		})
	//
	//		It("creates the network with mtu matching that of the host", func() {
	//			psCommand := `(Get-NetAdapter -Physical).Name`
	//			output, err := exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
	//			Expect(err).ToNot(HaveOccurred(), string(output))
	//			physicalNetworkName := strings.TrimSpace(string(output))
	//
	//			psCommand = fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias '%s').NlMtu`, physicalNetworkName)
	//			output, err = exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
	//			Expect(err).ToNot(HaveOccurred(), string(output))
	//			physicalMTU := strings.TrimSpace(string(output))
	//
	//			helpers.CreateNetwork(networkConfig, networkConfigFile)
	//
	//			psCommand = fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, networkConfig.NetworkName)
	//			output, err = exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
	//			Expect(err).ToNot(HaveOccurred(), string(output))
	//			virtualMTU := strings.TrimSpace(string(output))
	//
	//			Expect(virtualMTU).To(Equal(physicalMTU))
	//		})
	//
	//		Context("mtu is set in the config", func() {
	//			BeforeEach(func() {
	//				networkConfig.MTU = 1400
	//			})
	//
	//			It("creates the network with the configured mtu", func() {
	//				helpers.CreateNetwork(networkConfig, networkConfigFile)
	//
	//				psCommand := fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, networkConfig.NetworkName)
	//				output, err := exec.Command("powershell.exe", "-Command", psCommand).CombinedOutput()
	//				Expect(err).ToNot(HaveOccurred(), string(output))
	//				virtualMTU := strings.TrimSpace(string(output))
	//
	//				Expect(virtualMTU).To(Equal(strconv.Itoa(networkConfig.MTU)))
	//			})
	//		})
	//	})
	//
	//	Describe("Delete", func() {
	//		BeforeEach(func() {
	//			networkConfig = helpers.GenerateNetworkConfig()
	//			helpers.CreateNetwork(networkConfig, networkConfigFile)
	//
	//		})
	//
	//		It("deletes the NAT network", func() {
	//			helpers.DeleteNetwork(networkConfig, networkConfigFile)
	//
	//			psCommand := fmt.Sprintf(`(Get-NetAdapter -name "vEthernet (%s)").InterfaceAlias`, networkConfig.NetworkName)
	//			output, err := exec.Command("powershell.exe", "-command", psCommand).CombinedOutput()
	//			Expect(err).NotTo(HaveOccurred(), string(output))
	//			expectedOutput := fmt.Sprintf("Get-NetAdapter : No MSFT_NetAdapter objects found with property 'Name' equal to 'vEthernet (%s)'", networkConfig.NetworkName)
	//			Expect(strings.Replace(string(output), "\r\n", "", -1)).To(ContainSubstring(expectedOutput))
	//		})
	//	})

	Describe("Up", func() {
		var (
			containerId string
			bundlePath  string
			bundleSpec  specs.Spec
		)

		BeforeEach(func() {
			var err error
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			containerId = filepath.Base(bundlePath)

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
		})

		AfterEach(func() {
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
		})

		Context("default network config", func() {
			BeforeEach(func() {
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
				//				networkConfig = helpers.GenerateNetworkConfig()
				//				helpers.CreateNetwork(networkConfig, networkConfigFile)
			})

			AfterEach(func() {
				//			deleteContainerAndNetwork(containerId, networkConfig)
				helpers.NetworkDown(containerId, networkConfigFile)
				helpers.DeleteContainer(containerId)
				helpers.DeleteVolume(containerId)
			})

			XIt("sets the host MTU in the container", func() {
				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`, networkConfigFile)

				powershellCommand := `test-netconnection -port 50000 -computername 10.55.7.14`
				cmd := exec.Command("powershell.exe", "-Command", powershellCommand)
				output, err := cmd.CombinedOutput()
				fmt.Println(string(output))
				Expect(err).ToNot(HaveOccurred(), string(output))

				stdout, stderr, err := helpers.ExecInContainer(containerId, []string{"powershell.exe", "-command", powershellCommand}, false)
				fmt.Println(stdout.String())
				fmt.Println(stderr.String())
				Expect(err).NotTo(HaveOccurred())
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

				XIt("can hit a port on the container directly", func() {
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

					//		It("prints an empty list of mapped ports", func() {
					//			outputs := helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} }`, networkConfigFile)

					//			Expect(outputs.Properties.MappedPorts).To(Equal("[]"))
					//			Expect(outputs.Properties.DeprecatedHostIP).To(Equal("255.255.255.255"))

					//			_, network, err := net.ParseCIDR(networkConfig.SubnetRange)
					//			Expect(err).NotTo(HaveOccurred())
					//			ip := net.ParseIP(outputs.Properties.ContainerIP)
					//			Expect(ip).NotTo(BeNil())
					//			Expect(network.Contains(ip)).To(BeTrue())
					//		})
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

					errStr := "dial tcp 8.8.8.8:53: connectex: A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond."
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
							{Start: net.ParseIP("8.8.8.8"), End: net.ParseIP("8.8.8.8")},
						},
						Ports: []netrules.PortRange{{Start: 53, End: 53}},
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

						errStr := "dial tcp 8.8.8.8:53: connectex: A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond."
						Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))
					})

					It("cannot connect to a remote host over ICMP", func() {
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

					XContext("netout allows udp on port 53", func() {
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

						errStr := "dial tcp 8.8.4.4:53: connectex: A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond."
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

					XIt("can connect to a remote host over ICMP", func() {
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

						errStr := "dial tcp 8.8.8.8:53: connectex: A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond."
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

						//stdout, _, err = helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.8.8"}, false)
						//Expect(err).NotTo(HaveOccurred())
						//Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.8.8"))
						//Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 4, Lost = 0 (0% loss)"))
					})

					It("blocks access over all protocols to prohibited remote hosts", func() {
						helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

						stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "udp", "--addr", "8.8.4.4", "--port", "53"}, false)
						Expect(err).To(HaveOccurred())
						Expect(stdout.String()).To(ContainSubstring("failed to exchange: read udp"))
						Expect(stdout.String()).To(ContainSubstring("8.8.4.4:53: i/o timeout"))

						stdout, _, err = helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.4.4", "--port", "53"}, false)
						Expect(err).To(HaveOccurred())
						errStr := "dial tcp 8.8.4.4:53: connectex: A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond."
						Expect(strings.TrimSpace(stdout.String())).To(Equal(errStr))

						stdout, _, err = helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "icmp", "--addr", "8.8.4.4"}, false)
						Expect(err).To(HaveOccurred())
						Expect(stdout.String()).To(ContainSubstring("Ping statistics for 8.8.4.4"))
						Expect(stdout.String()).To(ContainSubstring("Packets: Sent = 4, Received = 0, Lost = 4 (100% loss)"))
					})
				})
			})
		})

		//		Context("custom MTU", func() {
		//			BeforeEach(func() {
		//				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		//				networkConfig = helpers.GenerateNetworkConfig()
		//				networkConfig.MTU = 1405
		//				helpers.CreateNetwork(networkConfig, networkConfigFile)
		//
		//			})
		//
		//			AfterEach(func() {
		//				deleteContainerAndNetwork(containerId, networkConfig)
		//			})
		//
		//			It("sets the network MTU on the internal container NIC", func() {
		//				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`, networkConfigFile)
		//
		//				stdout, _, err := helpers.ExecInContainer(containerId, []string{"powershell.exe", "-Command", `(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias "vEthernet*").NlMtu`}, false)
		//				Expect(err).NotTo(HaveOccurred())
		//				Expect(strings.TrimSpace(stdout.String())).To(Equal("1405"))
		//			})
		//		})
		//
		//		Context("custom DNS Servers", func() {
		//			BeforeEach(func() {
		//				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		//				networkConfig = helpers.GenerateNetworkConfig()
		//				networkConfig.DNSServers = []string{"8.8.8.8", "8.8.4.4"}
		//				helpers.CreateNetwork(networkConfig, networkConfigFile)
		//			})
		//
		//			AfterEach(func() {
		//				deleteContainerAndNetwork(containerId, networkConfig)
		//			})
		//
		//			It("uses those IP addresses as DNS servers", func() {
		//				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`, networkConfigFile)
		//
		//				stdout, _, err := helpers.ExecInContainer(containerId, []string{"powershell.exe", "-Command", `(Get-DnsClientServerAddress -InterfaceAlias 'vEthernet*' -AddressFamily IPv4).ServerAddresses -join ","`}, false)
		//				Expect(err).NotTo(HaveOccurred())
		//				Expect(strings.TrimSpace(stdout.String())).To(Equal("8.8.8.8,8.8.4.4"))
		//			})
		//
		//			It("allows traffic to those servers", func() {
		//				helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} ,"netin": []}`, networkConfigFile)
		//
		//				pid := helpers.GetContainerState(containerId).Pid
		//				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)
		//
		//				stdout, _, err := helpers.ExecInContainer(containerId, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "8.8.8.8", "--port", "53"}, false)
		//				Expect(err).NotTo(HaveOccurred())
		//				Expect(strings.TrimSpace(stdout.String())).To(Equal("connected to 8.8.8.8:53 over tcp"))
		//			})
		//		})
		//	})
	})

	//	Describe("Down", func() {
	//		var (
	//			containerId string
	//			bundlePath  string
	//			bundleSpec  specs.Spec
	//		)
	//
	//		BeforeEach(func() {
	//			var err error
	//			bundlePath, err = ioutil.TempDir("", "winccontainer")
	//			Expect(err).To(Succeed())
	//
	//			containerId = filepath.Base(bundlePath)
	//
	//			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))
	//
	//			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
	//			networkConfig = helpers.GenerateNetworkConfig()
	//			helpers.CreateNetwork(networkConfig, networkConfigFile)
	//
	//			output, err := exec.Command(wincNetworkBin, "--action", "create", "--configFile", networkConfigFile).CombinedOutput()
	//			Expect(err).NotTo(HaveOccurred(), string(output))
	//
	//			helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {}}`, networkConfigFile)
	//			Expect(len(allEndpoints(containerId))).To(Equal(1))
	//		})
	//
	//		AfterEach(func() {
	//			deleteContainerAndNetwork(containerId, networkConfig)
	//			Expect(os.RemoveAll(bundlePath)).To(Succeed())
	//		})
	//
	//		It("deletes the endpoint", func() {
	//			cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId)
	//			output, err := cmd.CombinedOutput()
	//			Expect(err).NotTo(HaveOccurred(), string(output))
	//			Expect(len(allEndpoints(containerId))).To(Equal(0))
	//			Expect(endpointExists(containerId)).To(BeFalse())
	//		})
	//
	//		It("deletes the associated firewall rules", func() {
	//			cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId)
	//			output, err := cmd.CombinedOutput()
	//			Expect(err).NotTo(HaveOccurred(), string(output))
	//
	//			getFirewallRule := fmt.Sprintf(`Get-NetFirewallRule -DisplayName "%s"`, containerId)
	//			output, err = exec.Command("powershell.exe", "-Command", getFirewallRule).CombinedOutput()
	//			Expect(err).To(HaveOccurred())
	//			expectedOutput := fmt.Sprintf(`Get-NetFirewallRule : No MSFT_NetFirewallRule objects found with property 'DisplayName' equal to '%s'`, containerId)
	//			Expect(strings.Replace(string(output), "\r\n", "", -1)).To(ContainSubstring(expectedOutput))
	//		})
	//
	//		Context("when the endpoint does not exist", func() {
	//			It("does nothing", func() {
	//				cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", "some-nonexistant-id")
	//				output, err := cmd.CombinedOutput()
	//				Expect(err).NotTo(HaveOccurred(), string(output))
	//			})
	//		})
	//
	//		Context("when the container is deleted before the endpoint", func() {
	//			BeforeEach(func() {
	//				output, err := exec.Command(wincBin, "delete", containerId).CombinedOutput()
	//				Expect(err).NotTo(HaveOccurred(), string(output))
	//			})
	//
	//			It("deletes the endpoint", func() {
	//				Expect(endpointExists(containerId)).To(BeTrue())
	//				cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId)
	//				output, err := cmd.CombinedOutput()
	//				Expect(err).NotTo(HaveOccurred(), string(output))
	//				Expect(endpointExists(containerId)).To(BeFalse())
	//			})
	//		})
	//	})

	Context("two containers are running", func() {
		var (
			bundlePath  string
			bundleSpec  specs.Spec
			containerId string

			bundlePath2   string
			bundleSpec2   specs.Spec
			containerId2  string
			containerPort string
			hostIP        string
			client        http.Client
		)

		BeforeEach(func() {
			var err error
			bundlePath, err = ioutil.TempDir("", "winccontainer")
			Expect(err).To(Succeed())

			containerId = filepath.Base(bundlePath)

			bundleSpec = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId))

			bundlePath2, err = ioutil.TempDir("", "winccontainer-2")
			Expect(err).NotTo(HaveOccurred())
			containerId2 = filepath.Base(bundlePath2)

			bundleSpec2 = helpers.GenerateRuntimeSpec(helpers.CreateVolume(rootfsURI, containerId2))

			containerPort = "12345"

			hostIP, err = localip.LocalIP()
			Expect(err).NotTo(HaveOccurred())

			client = *http.DefaultClient
			client.Timeout = 5 * time.Second

			//networkConfig = helpers.GenerateNetworkConfig()
			//helpers.CreateNetwork(networkConfig, networkConfigFile)

		})

		AfterEach(func() {
			helpers.NetworkDown(containerId2, networkConfigFile)
			helpers.DeleteContainer(containerId2)
			helpers.DeleteVolume(containerId2)

			helpers.NetworkDown(containerId, networkConfigFile)
			helpers.DeleteContainer(containerId)
			helpers.DeleteVolume(containerId)
			//deleteContainerAndNetwork(containerId, networkConfig)
			Expect(os.RemoveAll(bundlePath)).To(Succeed())
			Expect(os.RemoveAll(bundlePath2)).To(Succeed())
		})

		It("by default, it does not allow traffic between containers", func() {
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
			outputs := helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)
			hostIp := outputs.Properties.ContainerIP
			Expect(helpers.ContainerExists(containerId)).To(BeTrue())
			hostPort := findExternalPort(outputs.Properties.MappedPorts, containerPort)

			pid := helpers.GetContainerState(containerId).Pid
			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

			_, _, err := helpers.ExecInContainer(containerId, []string{"c:\\server.exe", containerPort}, true)
			Expect(err).NotTo(HaveOccurred())

			helpers.CreateContainer(bundleSpec2, bundlePath2, containerId2)
			helpers.NetworkUp(containerId2, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

			pid = helpers.GetContainerState(containerId2).Pid
			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)

			stdOut, _, err := helpers.ExecInContainer(containerId2, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", hostIp, "--port", strconv.Itoa(hostPort)}, false)
			Expect(err).To(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("connectex: A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond"))

			stdOut, _, err = helpers.ExecInContainer(containerId2, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", getContainerIp(containerId).String(), "--port", containerPort}, false)
			Expect(err).To(HaveOccurred())
			Expect(stdOut.String()).To(ContainSubstring("connectex: A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond"))
		})

		It("can route traffic to the remaining container after the other is deleted", func() {
			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
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

			helpers.CreateContainer(bundleSpec2, bundlePath2, containerId2)
			outputs = helpers.NetworkUp(containerId2, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)

			Expect(json.Unmarshal([]byte(outputs.Properties.MappedPorts), &mappedPorts)).To(Succeed())

			Expect(len(mappedPorts)).To(Equal(1))
			hostPort2 := mappedPorts[0].HostPort
			Expect(hostPort2).NotTo(Equal(hostPort1))

			pid = helpers.GetContainerState(containerId2).Pid
			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

			_, _, err = helpers.ExecInContainer(containerId2, []string{"c:\\server.exe", containerPort}, true)
			Expect(err).NotTo(HaveOccurred())

			resp, err = client.Get(fmt.Sprintf("http://%s:%d", hostIP, hostPort2))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			data, err = ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %s", containerPort)))

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

		Context("the containers are set up for c2c", func() {
			It("does c2c", func() {
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
				outputs := helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)

				mappedPorts := []netrules.PortMapping{}
				Expect(json.Unmarshal([]byte(outputs.Properties.MappedPorts), &mappedPorts)).To(Succeed())

				Expect(len(mappedPorts)).To(Equal(1))

				pid := helpers.GetContainerState(containerId).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

				_, _, err := helpers.ExecInContainer(containerId, []string{"c:\\server.exe", containerPort}, true)
				Expect(err).NotTo(HaveOccurred())

				helpers.CreateContainer(bundleSpec2, bundlePath2, containerId2)
				pid = helpers.GetContainerState(containerId2).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)

				containerIp := getContainerIp(containerId)
				cPort := uint16(mappedPorts[0].ContainerPort)

				netOutRule := netrules.NetOut{
					Networks: []netrules.IPRange{
						{Start: containerIp, End: containerIp},
					},
					Ports:    []netrules.PortRange{{Start: cPort, End: cPort}},
					Protocol: netrules.ProtocolTCP,
				}
				netOutRules, err := json.Marshal([]netrules.NetOut{netOutRule})
				Expect(err).NotTo(HaveOccurred())

				helpers.NetworkUp(containerId2, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(netOutRules)), networkConfigFile)

				stdout, _, err := helpers.ExecInContainer(containerId2, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", containerIp.String(), "--port", containerPort}, false)
				Expect(err).NotTo(HaveOccurred())

				Expect(strings.TrimSpace(stdout.String())).To(Equal(fmt.Sprintf("connected to %s:%s over tcp", containerIp.String(), containerPort)))
			})
		})

		//	Context("the max outgoing bandwidth is set in the config file", func() {
		//		var (
		//			serverURL         string
		//			clientNetOutRules []byte
		//		)
		//		const (
		//			tinyBandwidth  = 1024 * 1024
		//			giantBandwidth = 10 * 1024 * 1024
		//			fileSize       = 10 * 1024 * 1024
		//		)
		//		BeforeEach(func() {
		//			var err error

		//			helpers.CreateContainer(bundleSpec, bundlePath, containerId)
		//			outputs := helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)
		//			hostIp, err := localip.LocalIP()
		//			Expect(err).NotTo(HaveOccurred())
		//			Expect(helpers.ContainerExists(containerId)).To(BeTrue())
		//			hostPort := findExternalPort(outputs.Properties.MappedPorts, containerPort)
		//			serverURL = fmt.Sprintf("http://%s:%d", hostIp, hostPort)

		//			pid := helpers.GetContainerState(containerId).Pid
		//			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

		//			_, _, err = helpers.ExecInContainer(containerId, []string{"c:\\server.exe", containerPort}, true)
		//			Expect(err).NotTo(HaveOccurred())

		//			netOutRule := netrules.NetOut{
		//				Protocol: netrules.ProtocolAll,
		//				Networks: []netrules.IPRange{
		//					{Start: net.ParseIP(hostIp), End: net.ParseIP(hostIp)},
		//				},
		//				Ports: []netrules.PortRange{{Start: uint16(hostPort), End: uint16(hostPort)}},
		//			}
		//			clientNetOutRules, err = json.Marshal([]netrules.NetOut{netOutRule})
		//			Expect(err).NotTo(HaveOccurred())

		//			resp, err := http.Get(serverURL)
		//			Expect(err).NotTo(HaveOccurred())
		//			defer resp.Body.Close()
		//			data, err := ioutil.ReadAll(resp.Body)
		//			Expect(err).NotTo(HaveOccurred())
		//			Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %s", containerPort)))
		//		})

		//		It("applies the bandwidth limit on the container to outgoing traffic", func() {
		//			helpers.CreateContainer(bundleSpec2, bundlePath2, containerId2)

		//			pid := helpers.GetContainerState(containerId2).Pid
		//			helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "client.exe"), clientBin)

		//			networkConfig.MaximumOutgoingBandwidth = tinyBandwidth
		//			helpers.WriteNetworkConfig(networkConfig, networkConfigFile)
		//			helpers.NetworkUp(containerId2, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(clientNetOutRules)), networkConfigFile)

		//			tinyTime := uploadFile(containerId2, fileSize, serverURL)

		//			helpers.NetworkDown(containerId2, networkConfigFile)

		//			networkConfig.MaximumOutgoingBandwidth = giantBandwidth
		//			helpers.WriteNetworkConfig(networkConfig, networkConfigFile)
		//			helpers.NetworkUp(containerId2, fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": %s}`, string(clientNetOutRules)), networkConfigFile)

		//			giantTime := uploadFile(containerId2, fileSize, serverURL)

		//			Expect(tinyTime).To(BeNumerically(">", giantTime*7))
		//		})
		//	})

		Context("when the containers share a network namespace", func() {
			BeforeEach(func() {
				bundleSpec2.Windows.Network = &specs.WindowsNetwork{NetworkSharedContainerName: containerId}
				containerPort = "23456"
			})

			It("allows traffic between the containers", func() {
				helpers.CreateContainer(bundleSpec, bundlePath, containerId)
				helpers.NetworkUp(containerId, fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %s}]}`, 0, containerPort), networkConfigFile)

				pid := helpers.GetContainerState(containerId).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)

				_, _, err := helpers.ExecInContainer(containerId, []string{"c:\\server.exe", containerPort}, true)
				Expect(err).NotTo(HaveOccurred())

				helpers.CreateContainer(bundleSpec2, bundlePath2, containerId2)
				helpers.NetworkUp(containerId2, `{"Pid": 123, "Properties": {}}`, networkConfigFile)

				pid = helpers.GetContainerState(containerId2).Pid
				helpers.CopyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "netout.exe"), netoutBin)

				stdOut, _, err := helpers.ExecInContainer(containerId2, []string{"c:\\netout.exe", "--protocol", "tcp", "--addr", "127.0.0.1", "--port", containerPort}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(stdOut.String()).To(Equal(fmt.Sprintf("connected to 127.0.0.1:%s over tcp", containerPort)))
			})
			Context("when deleting the first container", func() {
				BeforeEach(func() {
					helpers.CreateContainer(bundleSpec, bundlePath, containerId)
					helpers.NetworkUp(containerId, `{"Pid": 123, "Properties": {} }`, networkConfigFile)
					helpers.CreateContainer(bundleSpec2, bundlePath2, containerId2)
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

	//	Context("when provided --log <log-file>", func() {
	//		var (
	//			logFile string
	//			tempDir string
	//		)
	//
	//		BeforeEach(func() {
	//			var err error
	//			tempDir, err = ioutil.TempDir("", "log-dir")
	//			Expect(err).NotTo(HaveOccurred())
	//
	//			logFile = filepath.Join(tempDir, "winc-network.log")
	//
	//			networkConfig = helpers.GenerateNetworkConfig()
	//		})
	//
	//		AfterEach(func() {
	//			helpers.DeleteNetwork(networkConfig, networkConfigFile)
	//
	//			Expect(os.RemoveAll(tempDir)).To(Succeed())
	//		})
	//
	//		Context("when the provided log file path does not exist", func() {
	//			BeforeEach(func() {
	//				logFile = filepath.Join(tempDir, "some-dir", "winc-network.log")
	//			})
	//
	//			It("creates the full path", func() {
	//				helpers.CreateNetwork(networkConfig, networkConfigFile, "--log", logFile)
	//
	//				Expect(logFile).To(BeAnExistingFile())
	//			})
	//		})
	//
	//		Context("when it runs successfully", func() {
	//			It("does not log to the specified file", func() {
	//				helpers.CreateNetwork(networkConfig, networkConfigFile, "--log", logFile)
	//
	//				contents, err := ioutil.ReadFile(logFile)
	//				Expect(err).NotTo(HaveOccurred())
	//
	//				Expect(string(contents)).To(BeEmpty())
	//			})
	//
	//			Context("when provided --debug", func() {
	//				It("outputs debug level logs", func() {
	//					helpers.CreateNetwork(networkConfig, networkConfigFile, "--log", logFile, "--debug")
	//
	//					contents, err := ioutil.ReadFile(logFile)
	//					Expect(err).NotTo(HaveOccurred())
	//
	//					Expect(string(contents)).NotTo(BeEmpty())
	//				})
	//			})
	//		})
	//
	//		Context("when it errors", func() {
	//			BeforeEach(func() {
	//				c, err := json.Marshal(networkConfig)
	//				Expect(err).NotTo(HaveOccurred())
	//				Expect(ioutil.WriteFile(networkConfigFile, c, 0644)).To(Succeed())
	//			})
	//
	//			It("logs errors to the specified file", func() {
	//				exec.Command(wincNetworkBin, "--action", "some-invalid-action", "--log", logFile).CombinedOutput()
	//
	//				contents, err := ioutil.ReadFile(logFile)
	//				Expect(err).NotTo(HaveOccurred())
	//
	//				Expect(string(contents)).NotTo(BeEmpty())
	//				Expect(string(contents)).To(ContainSubstring("some-invalid-action"))
	//			})
	//		})
	//	})
})

func uploadFile(containerId string, fileSize int, serverURL string) int {
	stdout, _, err := helpers.ExecInContainer(containerId, []string{"C:\\client.exe", serverURL, "upload", strconv.Itoa(fileSize)}, false)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	outputRegex := regexp.MustCompile(`uploaded in ([0-9]+) miliseconds`)
	match := outputRegex.FindStringSubmatch(strings.TrimSpace(stdout.String()))
	ExpectWithOffset(1, len(match)).To(Equal(2))
	uploadTime, err := strconv.Atoi(match[1])
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return uploadTime
}

func downloadFile(containerId string, fileSize int, serverURL string) int {
	stdout, _, err := helpers.ExecInContainer(containerId, []string{"C:\\client.exe", serverURL, "download", strconv.Itoa(fileSize)}, false)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	outputRegex := regexp.MustCompile(`downloaded in ([0-9]+) miliseconds`)
	match := outputRegex.FindStringSubmatch(strings.TrimSpace(stdout.String()))
	ExpectWithOffset(1, len(match)).To(Equal(2))
	downloadTime, err := strconv.Atoi(match[1])
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return downloadTime
}

func deleteContainerAndNetwork(id string, config network.Config) {
	helpers.NetworkDown(id, networkConfigFile)
	helpers.DeleteContainer(id)
	helpers.DeleteVolume(id)
	helpers.DeleteNetwork(config, networkConfigFile)
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
		if _, ok := err.(hcsshim.EndpointNotFoundError); ok {
			return false
		}

		Expect(err).NotTo(HaveOccurred())
	}

	return true
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

func findExternalPort(portMappings, containerPort string) int {
	var mappedPorts []netrules.PortMapping
	err := json.Unmarshal([]byte(portMappings), &mappedPorts)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	var externalPort, internalPort int
	internalPort, err = strconv.Atoi(containerPort)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	for _, v := range mappedPorts {
		if v.ContainerPort == uint32(internalPort) {
			externalPort = int(v.HostPort)
			break
		}
	}
	ExpectWithOffset(1, externalPort).ToNot(Equal(0))
	return externalPort
}
