package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"code.cloudfoundry.org/winc/network"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("up", func() {
	var (
		config      []byte
		containerId string
		bundleSpec  specs.Spec
		err         error
		stdOut      *bytes.Buffer
		stdErr      *bytes.Buffer
	)

	BeforeEach(func() {
		containerId = filepath.Base(bundlePath)
		bundleSpec = runtimeSpecGenerator(createSandbox("C:\\run\\winc", rootfsPath, containerId), containerId)
		config, err = json.Marshal(&bundleSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), config, 0666)).To(Succeed())

		err := exec.Command(wincBin, "create", "-b", bundlePath, containerId).Run()
		Expect(err).ToNot(HaveOccurred())

		stdOut = new(bytes.Buffer)
		stdErr = new(bytes.Buffer)
	})

	AfterEach(func() {
		err := exec.Command(wincBin, "delete", containerId).Run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec.Command(wincImageBin, "--store", "C:\\run\\winc", "delete", containerId).Run()).To(Succeed())
	})

	Context("a config file contains network MTU", func() {
		var (
			configFile string
			mtu        int
		)

		BeforeEach(func() {
			mtu = 1405
			dir, err := ioutil.TempDir("", "winc-network.config")
			Expect(err).NotTo(HaveOccurred())

			configFile = filepath.Join(dir, "winc-network.json")
		})

		JustBeforeEach(func() {
			Expect(ioutil.WriteFile(configFile, []byte(fmt.Sprintf(`{"mtu": %d}`, mtu)), 0644)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(filepath.Dir(configFile))).To(Succeed())
		})

		It("sets the network MTU on the internal container NIC", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": []}`)
			Expect(cmd.Run()).To(Succeed())

			cmd = exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", `(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias "vEthernet*").NlMtu`)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(string(output))).To(Equal("1405"))
		})

		Context("when the requested network MTU is over 1500", func() {
			BeforeEach(func() {
				mtu = 1505
			})

			It("returns an error", func() {
				cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
				cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": []}`)

				output, err := cmd.CombinedOutput()
				Expect(err).NotTo(Succeed())
				Expect(string(output)).To(Equal("networkUp: invalid mtu specified: 1505"))
			})
		})
	})

	Context("stdin contains a port mapping request", func() {
		It("prints the correct port mapping for the container", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)
			output, err := cmd.CombinedOutput()
			Expect(err).To(Succeed())

			regex := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[{\\"HostPort\\":\d+,\\"ContainerPort\\":8080}\]"}}`
			Expect(string(output)).To(MatchRegexp(regex))
		})

		It("outputs the host's public IP as the container IP", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)
			output, err := cmd.CombinedOutput()
			Expect(err).To(Succeed())

			regex := regexp.MustCompile(`"garden\.network\.container-ip":"(\d+\.\d+\.\d+\.\d+)"`)
			matches := regex.FindStringSubmatch(string(output))
			Expect(len(matches)).To(Equal(2))

			cmd = exec.Command("powershell", "-Command", "Get-NetIPAddress", matches[1])
			output, err = cmd.CombinedOutput()
			Expect(err).To(BeNil())
			Expect(string(output)).NotTo(ContainSubstring("Loopback"))
			Expect(string(output)).NotTo(ContainSubstring("HNS Internal NIC"))
			Expect(string(output)).To(MatchRegexp("AddressFamily.*IPv4"))
		})

		It("creates the correct urlacl in the container", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)
			Expect(cmd.Run()).To(Succeed())

			output, err := exec.Command(wincBin, "exec", containerId, "cmd.exe", "/C", "netsh http show urlacl url=http://*:8080/ | findstr User").CombinedOutput()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(output)).To(ContainSubstring("BUILTIN\\Users"))
		})
	})

	Context("stdin contains a port mapping request with two ports", func() {
		It("prints the correct port mapping for the container", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}, {"host_port": 0, "container_port": 2222}]}`)
			output, err := cmd.CombinedOutput()
			Expect(err).To(Succeed())

			regex := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[{\\"HostPort\\":\d+,\\"ContainerPort\\":8080},{\\"HostPort\\":\d+,\\"ContainerPort\\":2222}\]"}}`
			Expect(string(output)).To(MatchRegexp(regex))
		})
	})

	Context("stdin does not contain a port mapping request", func() {
		It("prints an empty list of mapped ports", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} }`)
			output, err := cmd.CombinedOutput()
			Expect(err).To(Succeed())

			regex := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[\]"}}`
			Expect(string(output)).To(MatchRegexp(regex))
		})
	})

	Context("stdin contains an invalid port mapping request", func() {
		It("errors", func() {
			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 1234}, {"host_port": 0, "container_port": 2222}]}`)
			session, err := gexec.Start(cmd, stdOut, stdErr)
			Expect(err).To(Succeed())

			Eventually(session).Should(gexec.Exit(1))
			Expect(stdErr.String()).To(ContainSubstring("invalid port mapping"))
		})
	})

	Context("stdin contains net out rules", func() {
		type firewallRuleRemoteIP struct {
			Value []string `json:"Value"`
		}

		type firewallRuleUDPTCP struct {
			Protocol   string `json:"Protocol"`
			RemotePort struct {
				Value []string `json:"Value"`
			} `json:"RemotePort"`
			RemoteIP struct {
				Value []string `json:"Value"`
			} `json:"RemoteIP"`
		}

		var (
			containerIp string
			protocol    network.Protocol
		)

		JustBeforeEach(func() {
			containerIp = getContainerIp(containerId).String()
			netOutRule := network.NetOutRule{
				Protocol: protocol,
				Networks: []network.IPRange{
					network.IPRangeFromIP(net.ParseIP("8.8.8.8")),
					network.IPRange{
						Start: net.ParseIP("10.0.0.0"),
						End:   net.ParseIP("13.0.0.0"),
					},
				},
				Ports: []network.PortRange{
					network.PortRangeFromPort(80),
					network.PortRange{
						Start: 8080,
						End:   8090,
					},
				},
			}
			netOutRuleStr, err := json.Marshal(&netOutRule)
			Expect(err).ToNot(HaveOccurred())

			cmd := exec.Command(wincNetworkBin, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(fmt.Sprintf(`{"Pid": 123, "Properties": {}, "netout_rules": [%s]}`, string(netOutRuleStr)))
			Expect(cmd.Run()).To(Succeed())
		})

		AfterEach(func() {
			Expect(exec.Command(wincNetworkBin, "--action", "down", "--handle", containerId).Run()).To(Succeed())
			parsedCmd := fmt.Sprintf(`Get-NetFirewallAddressFilter | ?{$_.LocalAddress -eq "%s"}`, containerIp)
			output, err := exec.Command("powershell.exe", "-Command", parsedCmd).CombinedOutput()
			Expect(err).To(Succeed())
			Expect(string(output)).To(BeEmpty())
		})

		Context("with a single port/ip and port/ip ranges", func() {
			Context("with the TCP protocol", func() {
				BeforeEach(func() {
					protocol = network.ProtocolTCP
				})

				It("creates the correct firewall rule", func() {
					var f firewallRuleUDPTCP
					getContainerFirewallRule(containerIp, &f)
					Expect(strings.ToUpper(f.Protocol)).To(Equal("TCP"))
					Expect(f.RemoteIP.Value).To(ConsistOf([]string{"8.8.8.8", "10.0.0.0-13.0.0.0"}))
					Expect(f.RemotePort.Value).To(ConsistOf([]string{"80", "8080-8090"}))
				})
			})

			Context("with the UDP protocol", func() {
				BeforeEach(func() {
					protocol = network.ProtocolUDP
				})

				It("creates the correct firewall rule", func() {
					var f firewallRuleUDPTCP
					getContainerFirewallRule(containerIp, &f)
					Expect(strings.ToUpper(f.Protocol)).To(Equal("UDP"))
					Expect(f.RemoteIP.Value).To(ConsistOf([]string{"8.8.8.8", "10.0.0.0-13.0.0.0"}))
					Expect(f.RemotePort.Value).To(ConsistOf([]string{"80", "8080-8090"}))
				})
			})

			Context("with the ICMP protocol", func() {
				BeforeEach(func() {
					protocol = network.ProtocolICMP
				})

				It("does not create a firewall rule", func() {
					noMatchingFirewallRule(containerIp)
				})
			})

			Context("with the ANY protocol", func() {
				type firewallRuleAll struct {
					Protocol string               `json:"Protocol"`
					RemoteIP firewallRuleRemoteIP `json:"RemoteIP"`
				}

				BeforeEach(func() {
					protocol = network.ProtocolAll
				})

				It("creates the correct firewall rule", func() {
					var f firewallRuleAll
					getContainerFirewallRule(containerIp, &f)
					Expect(strings.ToUpper(f.Protocol)).To(Equal("ANY"))
					Expect(f.RemoteIP.Value).To(ConsistOf([]string{"8.8.8.8", "10.0.0.0-13.0.0.0"}))
				})
			})
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

func noMatchingFirewallRule(containerIp string) {
	const getContainerFirewallRuleAddresses = `Get-NetFirewallAddressFilter | ?{$_.LocalAddress -eq "%s"} | ConvertTo-Json`
	const getContainerFirewallRulePorts = `Get-NetFirewallAddressFilter | ?{$_.LocalAddress -eq "%s"} | Get-NetFirewallRule | Get-NetFirewallPortFilter | ConvertTo-Json`

	getRemotePortsCmd := fmt.Sprintf(getContainerFirewallRulePorts, containerIp)
	output, err := exec.Command("powershell.exe", "-Command", getRemotePortsCmd).CombinedOutput()
	Expect(err).To(Succeed())
	Expect(string(output)).To(Equal(""))

	getRemoteAddressesCmd := fmt.Sprintf(getContainerFirewallRuleAddresses, containerIp)
	output, err = exec.Command("powershell.exe", "-Command", getRemoteAddressesCmd).CombinedOutput()
	Expect(err).To(Succeed())
	Expect(string(output)).To(Equal(""))
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
