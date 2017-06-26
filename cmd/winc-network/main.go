package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Microsoft/hcsshim"
)

type NetIn struct {
	HostPort      uint32 `json:"host_port"`
	ContainerPort uint32 `json:"container_port"`
}

type UpInputs struct {
	Pid        int
	Properties map[string]interface{}
	//NetOut     []garden.NetOutRule `json:"netout_rules"`
	NetIn []NetIn `json:"netin"`
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
		os.Exit(0)
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

	if len(inputs.NetIn) != 1 {
		return fmt.Errorf("invalid number of port mappings: %d", len(inputs.NetIn))
	}

	mapping := inputs.NetIn[0]

	if mapping.ContainerPort != 8080 || mapping.HostPort != 0 {
		return fmt.Errorf("invalid port mapping: %+v", mapping)
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

	var mappedPort NetIn
	natPolicy := hcsshim.NatPolicy{}

	for _, pol := range endpoint.Policies {
		if err := json.Unmarshal(pol, &natPolicy); err != nil {
			return err
		}
		if natPolicy.Type == "NAT" {
			mappedPort.HostPort = uint32(natPolicy.ExternalPort)
			mappedPort.ContainerPort = uint32(natPolicy.InternalPort)
			break
		}
	}

	upOutputs := UpOutputs{}
	upOutputs.Properties.ContainerIP = endpoint.IPAddress.String()
	upOutputs.Properties.DeprecatedHostIP = "255.255.255.255"

	portBytes, err := json.Marshal(mappedPort)
	if err != nil {
		return err
	}
	upOutputs.Properties.MappedPorts = string(portBytes)

	return json.NewEncoder(os.Stdout).Encode(upOutputs)
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
