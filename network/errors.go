package network

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
)

type NoNATNetworkError struct {
	Name string
}

func (e *NoNATNetworkError) Error() string {
	return fmt.Sprintf("could not load nat network: %s", e.Name)
}

type SameNATNetworkNameError struct {
	Name    string
	Subnets []hcsshim.Subnet
}

func (e *SameNATNetworkNameError) Error() string {
	return fmt.Sprintf("nat network %s exists with subnets %+v", e.Name, e.Subnets)
}
