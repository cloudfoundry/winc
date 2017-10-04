package network

import "github.com/Microsoft/hcsshim"

//go:generate counterfeiter . HCSClient
type HCSClient interface {
	GetHNSNetworkByName(string) (*hcsshim.HNSNetwork, error)
	CreateNetwork(*hcsshim.HNSNetwork) (*hcsshim.HNSNetwork, error)
	CreateEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
	GetHNSEndpointByID(string) (*hcsshim.HNSEndpoint, error)
	GetHNSEndpointByName(string) (*hcsshim.HNSEndpoint, error)
	DeleteEndpoint(*hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error)
}
