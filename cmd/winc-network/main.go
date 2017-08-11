package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/winc/network"
	"github.com/Microsoft/hcsshim"
)

type PortMapping struct {
	HostPort      uint32
	ContainerPort uint32
}

type UpInputs struct {
	Pid        int
	Properties map[string]interface{}
	NetOut     []network.NetOutRule `json:"netout_rules"`
	NetIn      []network.NetIn      `json:"netin"`
}

type UpOutputs struct {
	Properties struct {
		ContainerIP      string `json:"garden.network.container-ip"`
		DeprecatedHostIP string `json:"garden.network.host-ip"`
		MappedPorts      string `json:"garden.network.mapped-ports"`
	} `json:"properties"`
	DNSServers []string `json:"dns_servers,omitempty"`
}

func main() {
	action, handle, err := parseArgs(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid args: %s", err.Error())
		os.Exit(1)
	}

	if action == "up" {
		err := networkUp(handle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "networkUp: %s", err.Error())
			os.Exit(1)
		}
	} else if action == "down" {
		err := networkDown(handle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "networkDown: %s", err.Error())
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "invalid action: %s", action)
		os.Exit(1)
	}

}

func networkUp(containerId string) error {
	var inputs UpInputs
	if err := json.NewDecoder(os.Stdin).Decode(&inputs); err != nil {
		return err
	}

	if len(inputs.NetIn) > 2 {
		return fmt.Errorf("invalid number of port mappings: %d", len(inputs.NetIn))
	}

	container, err := hcsshim.OpenContainer(containerId)
	if err != nil {
		return err
	}

	stats, err := container.Statistics()
	if err != nil {
		return err
	}
	if len(stats.Network) != 1 {
		return fmt.Errorf("invalid number of container endpoints: %d", len(stats.Network))
	}

	endpointId := stats.Network[0].EndpointId
	endpoint, err := hcsshim.GetHNSEndpointByID(endpointId)
	if err != nil {
		return err
	}

	upOutputs := UpOutputs{}
	upOutputs.Properties.ContainerIP, err = localip.LocalIP()
	if err != nil {
		return err
	}
	upOutputs.Properties.DeprecatedHostIP = "255.255.255.255"

	mappedPorts := []PortMapping{}

	for _, mapping := range inputs.NetIn {
		if (mapping.ContainerPort != 8080 && mapping.ContainerPort != 2222) || mapping.HostPort != 0 {
			return fmt.Errorf("invalid port mapping: %+v", mapping)
		}

		natPolicy := hcsshim.NatPolicy{}

		for _, pol := range endpoint.Policies {
			if err := json.Unmarshal(pol, &natPolicy); err != nil {
				return err
			}
			if natPolicy.Type == "NAT" && uint32(natPolicy.InternalPort) == mapping.ContainerPort {
				mappedPort := PortMapping{
					ContainerPort: uint32(natPolicy.InternalPort),
					HostPort:      uint32(natPolicy.ExternalPort),
				}

				mappedPorts = append(mappedPorts, mappedPort)
				break
			}
		}
	}

	portBytes, err := json.Marshal(mappedPorts)
	if err != nil {
		return err
	}
	upOutputs.Properties.MappedPorts = string(portBytes)

	for _, netOut := range inputs.NetOut {
		netShArgs := []string{
			"advfirewall", "firewall", "add", "rule",
			fmt.Sprintf(`name="%s"`, containerId),
			"dir=out",
			"action=allow",
			fmt.Sprintf("localip=%s", endpoint.IPAddress.String()),
			fmt.Sprintf("remoteip=%s", network.ParseIPRange(netOut.Networks)),
		}

		var protocol string
		switch netOut.Protocol {
		case network.ProtocolTCP:
			protocol = "TCP"
			netShArgs = append(netShArgs, fmt.Sprintf("remoteport=%s", network.ParsePortRange(netOut.Ports)))
		case network.ProtocolUDP:
			protocol = "UDP"
			netShArgs = append(netShArgs, fmt.Sprintf("remoteport=%s", network.ParsePortRange(netOut.Ports)))
		case network.ProtocolAll:
			protocol = "ANY"
		default:
		}

		if protocol == "" {
			return errors.New("invalid protocol")
		}

		netShArgs = append(netShArgs, fmt.Sprintf("protocol=%s", protocol))

		err := exec.Command("netsh", netShArgs...).Run()
		if err != nil {
			return err
		}
	}

	return json.NewEncoder(os.Stdout).Encode(upOutputs)
}

func networkDown(containerId string) error {
	netShArgs := []string{
		"advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf(`name="%s"`, containerId),
	}

	_ = exec.Command("netsh", netShArgs...).Run()

	return nil
}

func parseArgs(allArgs []string) (string, string, error) {
	var action, handle string
	flagSet := flag.NewFlagSet("", flag.ContinueOnError)

	flagSet.StringVar(&action, "action", "", "")
	flagSet.StringVar(&handle, "handle", "", "")

	err := flagSet.Parse(allArgs[1:])
	if err != nil {
		return "", "", err
	}

	if handle == "" {
		return "", "", fmt.Errorf("missing required flag 'handle'")
	}

	if action == "" {
		return "", "", fmt.Errorf("missing required flag 'action'")
	}

	return action, handle, nil
}
