package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"code.cloudfoundry.org/winc/netrules"
	"code.cloudfoundry.org/winc/network"

	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("up", func() {
	var (
		config        []byte
		containerId   string
		bundleSpec    specs.Spec
		configFile    string
		networkConfig network.Config
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

		_, insiderPreview := os.LookupEnv("INSIDER_PREVIEW")
		networkConfig = network.Config{
			InsiderPreview: insiderPreview,
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
		output, err := exec.Command(wincBin, "delete", containerId).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
		output, err = exec.Command(wincImageBin, "--store", "C:\\run\\winc", "delete", containerId).CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), string(output))
		Expect(os.RemoveAll(filepath.Dir(configFile))).To(Succeed())
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

				cmd = exec.Command("powershell.exe", "-Command", `(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias "vEthernet*").NlMtu`)
				output, err = cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				hostMTU := strings.TrimSpace(string(output))

				cmd = exec.Command(wincBin, "exec", containerId, "powershell.exe", "-Command", `(Get-Netipinterface -AddressFamily ipv4 -InterfaceAlias "vEthernet*").NlMtu`)
				output, err = cmd.CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), string(output))
				Expect(strings.TrimSpace(string(output))).To(Equal(hostMTU))
			})
		})
	})

	Context("stdin contains a port mapping request", func() {
		It("prints the correct port mapping for the container", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))

			regex := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[{\\"HostPort\\":\d+,\\"ContainerPort\\":8080}\]"}}`
			Expect(string(output)).To(MatchRegexp(regex))
		})

		It("outputs the host's public IP as the container IP", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 8080}]}`)
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))

			regex := regexp.MustCompile(`"garden\.network\.container-ip":"(\d+\.\d+\.\d+\.\d+)"`)
			matches := regex.FindStringSubmatch(string(output))
			Expect(len(matches)).To(Equal(2))

			cmd = exec.Command("powershell", "-Command", "Get-NetIPAddress", matches[1])
			output, err = cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))
			Expect(string(output)).NotTo(ContainSubstring("Loopback"))
			Expect(string(output)).NotTo(ContainSubstring("HNS Internal NIC"))
			Expect(string(output)).To(MatchRegexp("AddressFamily.*IPv4"))
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
	})

	Context("stdin does not contain a port mapping request", func() {
		It("prints an empty list of mapped ports", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} }`)
			output, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred(), string(output))

			regex := `{"properties":{"garden\.network\.container-ip":"\d+\.\d+\.\d+\.\d+","garden\.network\.host-ip":"255\.255\.255\.255","garden\.network\.mapped-ports":"\[\]"}}`
			Expect(string(output)).To(MatchRegexp(regex))
		})
	})

	Context("stdin contains an invalid port mapping request", func() {
		It("errors", func() {
			cmd := exec.Command(wincNetworkBin, "--configFile", configFile, "--action", "up", "--handle", containerId)
			cmd.Stdin = strings.NewReader(`{"Pid": 123, "Properties": {} ,"netin": [{"host_port": 0, "container_port": 1234}, {"host_port": 0, "container_port": 2222}]}`)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).To(Succeed())

			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Err.Contents())).To(ContainSubstring("invalid port mapping"))
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
			containerIp = getContainerIp(containerId).String()
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
