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
	"strconv"
	"strings"

	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/network"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("up", func() {
	var (
		config          []byte
		containerId     string
		bundleSpec      specs.Spec
		configFile      string
		networkConfig   network.Config
		containerExists bool
	)

	BeforeEach(func() {
		containerId = filepath.Base(bundlePath)
		bundleSpec = runtimeSpecGenerator(createSandbox("C:\\run\\winc", rootfsPath, containerId), containerId)
		var err error
		config, err = json.Marshal(&bundleSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())

		output, err := exec.Command(wincBin, "create", "-b", bundlePath, containerId).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
		containerExists = true

		networkConfig = network.Config{
			SubnetRange:    subnetRange,
			GatewayAddress: gatewayAddress,
			NetworkName:    gatewayAddress,
		}
	})

	JustBeforeEach(func() {
		dir, err := ioutil.TempDir("", "winc-network.config")
		Expect(err).NotTo(HaveOccurred())

		configFile = filepath.Join(dir, "winc-network.json")
		c, err := json.Marshal(networkConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(configFile, c, 0644)).To(Succeed())
	})

	AfterEach(func() {
		output, err := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "down", "--handle", containerId).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
		Expect(endpointExists(containerId)).To(BeFalse())

		if containerExists {
			output, err = exec.Command(wincBin, "delete", containerId).CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
		}

		output, err = exec.Command(wincImageBin, "--store", "C:\\run\\winc", "delete", containerId).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))

		Expect(os.RemoveAll(filepath.Dir(configFile))).To(Succeed())
	})

	Context("network down", func() {
		JustBeforeEach(func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {}}`)
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			Expect(len(allEndpoints(containerId))).To(Equal(1))
		})

		It("deletes the endpoint", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "down", "--handle", containerId)
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			Expect(len(allEndpoints(containerId))).To(Equal(0))
			Expect(endpointExists(containerId)).To(BeFalse())
		})

		Context("when the endpoint does not exist", func() {
			It("does nothing", func() {
				cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "down", "--handle", "some-nonexistant-id")
				output, err := cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
			})
		})

		Context("when the container is deleted before the endpoint", func() {
			JustBeforeEach(func() {
				output, err := exec.Command(wincBin, "delete", containerId).CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
			})

			It("deletes the endpoint", func() {
				Expect(endpointExists(containerId)).To(BeTrue())
				cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "down", "--handle", containerId)
				output, err := cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				Expect(endpointExists(containerId)).To(BeFalse())
				containerExists = false
			})
		})
	})

	Context("the config file contains DNSServers", func() {
		BeforeEach(func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", networkConfigFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": []}`)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
		})

		It("uses those IP addresses as DNS servers", func() {
			cmd := exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", `(Get-DnsClientServerAddress -InterfaceAlias 'vEthernet*' -AddressFamily IPv4).ServerAddresses -join ","`)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			Expect(strings.TrimSpace(string(output))).To(Equal("1.1.1.1,2.2.2.2"))
		})
	})

	Context("a config file contains network MTU", func() {
		BeforeEach(func() {
			networkConfig.MTU = 1405
		})

		It("sets the network MTU on the internal container NIC", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": []}`)
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))

			cmd = exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", `(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias "vEthernet*").NlMtu`)
			output, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), string(output))
			Expect(strings.TrimSpace(string(output))).To(Equal("1405"))
		})

		Context("when the requested MTU is 0", func() {
			BeforeEach(func() {
				networkConfig.MTU = 0
			})

			It("sets the host MTU in the container", func() {
				cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
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
		})
	})

	Context("stdin contains a net in rule", func() {
		var (
			hostPort1 int
			hostPort2 int
			hostIp    string
			port1     int
			port2     int
			upOutput  network.UpOutputs
		)

		type portMapping struct {
			HostPort      int
			ContainerPort int
		}

		JustBeforeEach(func() {
			port1 = 12345
			port2 = 9876

			pid := getContainerState(containerId).Pid
			Expect(copyFile(filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "server.exe"), serverBin)).To(Succeed())

			cmd := exec.Command(wincBin, "exec", "-d", containerId, "c:\\server.exe", strconv.Itoa(port1))
			Expect(cmd.Run()).To(Succeed())

			cmd = exec.Command(wincBin, "exec", "-d", containerId, "c:\\server.exe", strconv.Itoa(port2))
			Expect(cmd.Run()).To(Succeed())

			hostPort2 = randomPort()
		})

		It("generates the correct port mappings and binds them to the container", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(fmt.Sprintf(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": %d},{"host_port": %d, "container_port": %d}]}`, port1, hostPort2, port2))
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			Expect(json.Unmarshal(output, &upOutput)).To(Succeed())

			mappedPorts := []portMapping{}
			Expect(json.Unmarshal([]byte(upOutput.Properties.MappedPorts), &mappedPorts)).To(Succeed())

			Expect(len(mappedPorts)).To(Equal(2))

			Expect(mappedPorts[0].ContainerPort).To(Equal(port1))
			Expect(mappedPorts[0].HostPort).NotTo(Equal(0))

			Expect(mappedPorts[1].ContainerPort).To(Equal(port2))
			Expect(mappedPorts[1].HostPort).To(Equal(hostPort2))

			hostPort1 = mappedPorts[0].HostPort

			hostIp = upOutput.Properties.ContainerIP
			resp, err := http.Get(fmt.Sprintf("http://%s:%d", hostIp, hostPort1))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			data, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %d", port1)))

			resp2, err := http.Get(fmt.Sprintf("http://%s:%d", hostIp, hostPort2))
			Expect(err).NotTo(HaveOccurred())
			defer resp2.Body.Close()

			data, err = ioutil.ReadAll(resp2.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(fmt.Sprintf("Response from server on port %d", port2)))
		})

		It("creates the correct urlacl in the container", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))

			output, err = exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "netsh http show urlacl url=http://*:8080/ | findstr User").CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			Expect(string(output)).To(ContainSubstring("BUILTIN\\Users"))
		})

		Context("stdin does not contain a port mapping request", func() {
			It("prints an empty list of mapped ports", func() {
				cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
				cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} }`)
				output, err := cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				Expect(json.Unmarshal(output, &upOutput)).To(Succeed())
				Expect(upOutput.Properties.MappedPorts).To(Equal("[]"))

				regex := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[\]"}}`
				Expect(string(output)).To(MatchRegexp(regex))
			})
		})
	})

	Context("stdin contains net out rules", func() {
		type firewallRule struct {
			Protocol   string `json:"Protocol"`
			RemotePort string `json:"RemotePort"`
			RemoteIP   string `json:"RemoteIP"`
		}

		var containerIp string

		JustBeforeEach(func() {
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
			Expect(err).ToNot(HaveOccurred())

			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": [%s]}`, string(netOutRuleStr)))
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))

			containerIp = getContainerIp(containerId).String()
		})

		AfterEach(func() {
			Expect(exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "down", "--handle", containerId).Run()).To(Succeed())
			parsedCmd := fmt.Sprintf(`Get-NetFirewallAddressFilter | ?{$_.LocalAddress -eq "%s"}`, containerIp)
			output, err := exec.Command("powershell.exe", "-Command", parsedCmd).CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			Expect(string(output)).To(BeEmpty())
		})

		It("creates the correct firewall rule", func() {
			var f firewallRule
			getContainerFirewallRule(containerIp, &f)
			Expect(strings.ToUpper(f.Protocol)).To(Equal("TCP"))
			Expect(f.RemoteIP).To(Equal("10.0.0.0-13.0.0.0"))
			Expect(f.RemotePort).To(Equal("8080-8090"))
		})
	})
})

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
	Expect(err).ToNot(HaveOccurred(), "no containers with id: "+containerId)

	stats, err := container.Statistics()
	Expect(err).ToNot(HaveOccurred())

	Expect(stats.Network).ToNot(BeEmpty(), "container has no networks attached: "+containerId)
	endpoint, err := hcsshim.GetHNSEndpointByID(stats.Network[0].EndpointId)
	Expect(err).ToNot(HaveOccurred())

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

type portMapping struct {
	HostPort      int
	ContainerPort int
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
