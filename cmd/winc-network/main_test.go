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
				cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
				cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": []}`)
				output, err := cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))

				powershellCommand := fmt.Sprintf(`(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet (%s)').NlMtu`, gatewayAddress)
				cmd = exec.Command("powershell.exe", "-Command", powershellCommand)
				output, err = cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				hostMTU := strings.TrimSpace(string(output))

				cmd = exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", "(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias 'vEthernet *').NlMtu")
				output, err = cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				Expect(strings.TrimSpace(string(output))).To(Equal(hostMTU))
			})

			Context("stdin contains a net in rule", func() {
				var (
					containerPort1 int
					containerPort2 int
				)

				BeforeEach(func() {
					containerPort1 = 12345
					containerPort2 = 9876

					pid := getContainerState(containerId).Pid
					Expect(copyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)).To(Succeed())

					cmd := exec.Command(wincBin, "exec", "-d", containerId, "c:\\server.exe", strconv.Itoa(containerPort1))
					Expect(cmd.Run()).To(Succeed())

					cmd = exec.Command(wincBin, "exec", "-d", containerId, "c:\\server.exe", strconv.Itoa(containerPort2))
					Expect(cmd.Run()).To(Succeed())
				})

				It("generates the correct port mappings and binds them to the container", func() {
					hostPort1 := 0
					hostPort2 := randomPort()
					cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
					cmd.Stdin = strings.NewReader(fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": %d, "container_port": %d},{"host_port": %d, "container_port": %d}]}`, hostPort1, containerPort1, hostPort2, containerPort2))
					output, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), string(output))
					var upOutput network.UpOutputs
					Expect(json.Unmarshal(output, &upOutput)).To(Succeed())

					type portMapping struct {
						HostPort      int
						ContainerPort int
					}
					mappedPorts := []portMapping{}
					Expect(json.Unmarshal([]byte(upOutput.Properties.MappedPorts), &mappedPorts)).To(Succeed())

					Expect(len(mappedPorts)).To(Equal(2))

					Expect(mappedPorts[0].ContainerPort).To(Equal(containerPort1))
					Expect(mappedPorts[0].HostPort).NotTo(Equal(hostPort1))

					Expect(mappedPorts[1].ContainerPort).To(Equal(containerPort2))
					Expect(mappedPorts[1].HostPort).To(Equal(hostPort2))

					hostPort1 = mappedPorts[0].HostPort

					hostIp := upOutput.Properties.ContainerIP
					resp, err := http.Get(fmt.Sprintf("http://%s:%d", hostIp, hostPort1))
					Expect(err).NotTo(HaveOccurred())
					defer resp.Body.Close()

					data, err := ioutil.ReadAll(resp.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %d", containerPort1)))

					resp2, err := http.Get(fmt.Sprintf("http://%s:%d", hostIp, hostPort2))
					Expect(err).NotTo(HaveOccurred())
					defer resp2.Body.Close()

					data, err = ioutil.ReadAll(resp2.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %d", containerPort2)))
				})

				It("creates the correct urlacl in the container", func() {
					cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
					cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)
					output, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), string(output))

					output, err = exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "netsh http show urlacl url=http://*:8080/ | findstr User").CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), string(output))
					Expect(string(output)).To(ContainSubstring("BUILTIN\\Users"))
				})

				Context("stdin does not contain a port mapping request", func() {
					It("prints an empty list of mapped ports", func() {
						cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
						cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} }`)
						output, err := cmd.CombinedOutput()
						Expect(err).NotTo(HaveOccurred(), string(output))
						var upOutput network.UpOutputs
						Expect(json.Unmarshal(output, &upOutput)).To(Succeed())
						Expect(upOutput.Properties.MappedPorts).To(Equal("[]"))

						regex := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[\]"}}`
						Expect(string(output)).To(MatchRegexp(regex))
					})
				})
			})

			Context("stdin contains net out rules", func() {
				var containerIp string

				BeforeEach(func() {
					netOutRule := netrules.NetOut{
						Protocol: netrules.ProtocolTCP,
						Networks: []netrules.IPRange{
							netrules.IPRange{
								Start: net.ParseIP("10.0.0.0"),
								End:   net.ParseIP("13.0.0.0"),
							},
						},
						Ports: []netrules.PortRange{
							netrules.PortRange{
								Start: 8080,
								End:   8090,
							},
						},
					}
					netOutRuleStr, err := json.Marshal(&netOutRule)
					Expect(err).NotTo(HaveOccurred())

					cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
					cmd.Stdin = strings.NewReader(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": [%s]}`, string(netOutRuleStr)))
					output, err := cmd.CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), string(output))

					containerIp = getContainerIp(containerId).String()
				})

				AfterEach(func() {
					Expect(exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "down", "--handle", containerId).Run()).To(Succeed())
					parsedCmd := fmt.Sprintf(`Get-NetFirewallAddressFilter | ?{$_.LocalAddress -eq "%s"}`, containerIp)
					output, err := exec.Command("powershell.exe", "-Command", parsedCmd).CombinedOutput()
					Expect(err).NotTo(HaveOccurred(), string(output))
					Expect(string(output)).To(BeEmpty())
				})

				It("creates the correct firewall rule", func() {
					var firewallRule struct {
						Protocol   string `json:"Protocol"`
						RemotePort string `json:"RemotePort"`
						RemoteIP   string `json:"RemoteIP"`
					}
					getContainerFirewallRule(containerIp, &firewallRule)
					Expect(strings.ToUpper(firewallRule.Protocol)).To(Equal("TCP"))
					Expect(firewallRule.RemoteIP).To(Equal("10.0.0.0-13.0.0.0"))
					Expect(firewallRule.RemotePort).To(Equal("8080-8090"))
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
				cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
				cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": []}`)
				output, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))

				cmd = exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", `(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias "vEthernet*").NlMtu`)
				output, err = cmd.CombinedOutput()
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
				cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
				cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": []}`)
				output, err := cmd.CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))

				cmd = exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", `(Get-DnsClientServerAddress -InterfaceAlias 'vEthernet*' -AddressFamily IPv4).ServerAddresses -join ","`)
				output, err = cmd.CombinedOutput()
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

			cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {}}`)
			output, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
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

func createNetwork(config network.Config, extraArgs ...string) {
	c, err := json.Marshal(config)
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(networkConfigFile, c, 0644)).To(Succeed())

	args := []string{"--action", "create", "--configFile", networkConfigFile}
	args = append(args, extraArgs...)
	output, err := exec.Command(wincNetworkBin, args...).CombinedOutput()
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

func deleteNetwork() {
	output, err := exec.Command(wincNetworkBin, "--action", "delete", "--configFile", networkConfigFile).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func getContainerFirewallRule(containerIp string, ruleInfo interface{}) {
	const getContainerFirewallRuleAddresses = `Get-NetFirewallAddressFilter | ?{$_.LocalAddress -eq "%s"} | ConvertTo-Json`
	const getContainerFirewallRulePorts = `Get-NetFirewallAddressFilter | ?{$_.LocalAddress -eq "%s"} | Get-NetFirewallRule | Get-NetFirewallPortFilter | ConvertTo-Json`

	getRemotePortsCmd := fmt.Sprintf(getContainerFirewallRulePorts, containerIp)
	output, err := exec.Command("powershell.exe", "-Command", getRemotePortsCmd).CombinedOutput()
	Expect(err).To(Succeed())
	Expect(json.Unmarshal(output, ruleInfo)).To(Succeed())

	getRemoteAddressesCmd := fmt.Sprintf(getContainerFirewallRuleAddresses, containerIp)
	output, err = exec.Command("powershell.exe", "-Command", getRemoteAddressesCmd).CombinedOutput()
	Expect(err).To(Succeed())
	Expect(json.Unmarshal(output, ruleInfo)).To(Succeed())
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
	gatewayAddress := fmt.Sprintf("172.0.%d.1", randomOctet)
	subnet := fmt.Sprintf("172.0.%d.0/30", randomOctet)
	return subnet, gatewayAddress
}
