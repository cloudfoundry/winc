package netrules

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim"
)

//go:generate counterfeiter . NetShRunner
type NetShRunner interface {
	RunContainer([]string) error
	RunHost([]string) ([]byte, error)
}

type Applier struct {
	netSh          NetShRunner
	id             string
	insiderPreview bool
	networkName    string
}

func NewApplier(netSh NetShRunner, containerId string, insiderPreview bool, networkName string) *Applier {
	return &Applier{
		netSh:          netSh,
		id:             containerId,
		insiderPreview: insiderPreview,
		networkName:    networkName,
	}
}

func (a *Applier) In(rule NetIn, endpoint *hcsshim.HNSEndpoint) (PortMapping, error) {
	portMapping := PortMapping{}

	if (rule.ContainerPort != 8080 && rule.ContainerPort != 2222) || rule.HostPort != 0 {
		return portMapping, fmt.Errorf("invalid port mapping: host %d, container %d", rule.HostPort, rule.ContainerPort)
	}

	for _, pol := range endpoint.Policies {
		natPolicy := hcsshim.NatPolicy{}
		if err := json.Unmarshal(pol, &natPolicy); err != nil {
			return portMapping, err
		}
		if natPolicy.Type == "NAT" && uint32(natPolicy.InternalPort) == rule.ContainerPort {
			portMapping = PortMapping{
				ContainerPort: uint32(natPolicy.InternalPort),
				HostPort:      uint32(natPolicy.ExternalPort),
			}

			break
		}
	}

	if err := a.openPort(portMapping.ContainerPort); err != nil {
		return portMapping, err
	}

	return portMapping, nil
}

func (a *Applier) Out(rule NetOut, endpoint *hcsshim.HNSEndpoint) error {
	netShArgs := []string{
		"advfirewall", "firewall", "add", "rule",
		fmt.Sprintf(`name="%s"`, a.id),
		"dir=out",
		"action=allow",
		fmt.Sprintf("localip=%s", endpoint.IPAddress.String()),
		fmt.Sprintf("remoteip=%s", firewallRuleIPRange(rule.Networks)),
	}

	var protocol string
	switch rule.Protocol {
	case ProtocolTCP:
		protocol = "TCP"
		netShArgs = append(netShArgs, fmt.Sprintf("remoteport=%s", firewallRulePortRange(rule.Ports)))
	case ProtocolUDP:
		protocol = "UDP"
		netShArgs = append(netShArgs, fmt.Sprintf("remoteport=%s", firewallRulePortRange(rule.Ports)))
	case ProtocolICMP:
		protocol = "ICMP"
	case ProtocolAll:
		protocol = "ANY"
	default:
	}

	if protocol == "ICMP" {
		return nil
	}

	if protocol == "" {
		return fmt.Errorf("invalid protocol: %d", rule.Protocol)
	}

	netShArgs = append(netShArgs, fmt.Sprintf("protocol=%s", protocol))

	_, err := a.netSh.RunHost(netShArgs)
	return err
}

func (a *Applier) MTU(interfaceInnerId string, mtu int) error {
	var err error
	if mtu == 0 {
		mtu, err = a.getHostMTU()
		if err != nil {
			return err
		}
	}

	if mtu > 1500 {
		return fmt.Errorf("invalid mtu specified: %d", mtu)
	}

	interfaceId := fmt.Sprintf(`"vEthernet (Container NIC %s)"`, strings.Split(interfaceInnerId, "-")[0])
	if a.insiderPreview {
		interfaceId = fmt.Sprintf(`"vEthernet (%s)"`, interfaceInnerId)
	}
	args := []string{"interface", "ipv4", "set", "subinterface", interfaceId, fmt.Sprintf("mtu=%d", mtu), "store=persistent"}

	return a.netSh.RunContainer(args)
}

func (a *Applier) openPort(port uint32) error {
	args := []string{"http", "add", "urlacl", fmt.Sprintf("url=http://*:%d/", port), "user=Users"}
	return a.netSh.RunContainer(args)
}

func (a *Applier) Cleanup() error {
	existsArgs := []string{"advfirewall", "firewall", "show", "rule", fmt.Sprintf(`name="%s"`, a.id)}
	_, err := a.netSh.RunHost(existsArgs)
	if err != nil {
		return nil
	}

	deleteArgs := []string{"advfirewall", "firewall", "delete", "rule", fmt.Sprintf(`name="%s"`, a.id)}

	_, err = a.netSh.RunHost(deleteArgs)
	return err
}

func (a *Applier) getHostMTU() (int, error) {
	interfaceId := "vEthernet (HNS Internal NIC)"
	if a.insiderPreview {
		interfaceId = fmt.Sprintf("vEthernet (%s)", a.networkName)
	}
	output, err := a.netSh.RunHost([]string{"interface", "ipv4", "show", "subinterface", "interface=" + interfaceId})
	if err != nil {
		return 0, err
	}

	mtuRegex := regexp.MustCompile("\\d+")
	mtuBytes := mtuRegex.Find(output)
	if mtuBytes == nil {
		return 0, errors.New("could not obtain host MTU")
	}

	hostMTU, err := strconv.Atoi(string(mtuBytes))
	if err != nil {
		return 0, errors.New("could not obtain host MTU")
	}

	return hostMTU, nil
}
