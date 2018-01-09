package netinterface

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

type NetInterface struct{}

type InterfaceNotFoundError struct {
	alias string
}

func (e *InterfaceNotFoundError) Error() string {
	return fmt.Sprintf("interface %s not found", e.alias)
}

type InterfaceForIPNotFoundError struct {
	ip string
}

func (e *InterfaceForIPNotFoundError) Error() string {
	return fmt.Sprintf("interface for ip %s not found", e.ip)
}

var (
	iphlpapi            = windows.NewLazySystemDLL("iphlpapi.dll")
	getIpInterfaceEntry = iphlpapi.NewProc("GetIpInterfaceEntry")
	setIpInterfaceEntry = iphlpapi.NewProc("SetIpInterfaceEntry")
)

// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365915(v=vs.85).aspx
const GAA_FLAG_INCLUDE_ALL_COMPARTMENTS = 0x200

// https://msdn.microsoft.com/en-us/library/windows/desktop/aa366320(v=vs.85).aspx
type NET_LUID struct {
	Value uint64
}

// https://msdn.microsoft.com/en-us/library/windows/desktop/aa814496(v=vs.85).aspx
type MIB_IPINTERFACE_ROW struct {
	Family                               uint32
	InterfaceLuid                        NET_LUID
	InterfaceIndex                       uint32
	MaxReassemblySize                    uint32
	InterfaceIdentifier                  uint64
	MinRouterAdvertisementInterval       uint32
	MaxRouterAdvertisementInterval       uint32
	AdvertisingEnabled                   bool
	ForwardingEnabled                    bool
	WeakHostSend                         bool
	WeakHostReceive                      bool
	UseAutomaticMetric                   bool
	UseNeighborUnreachabilityDetection   bool
	ManagedAddressConfigurationSupported bool
	OtherStatefulConfigurationSupported  bool
	AdvertiseDefaultRoute                bool
	RouterDiscoveryBehavior              uint32
	DadTransmits                         uint32
	BaseReachableTime                    uint32
	RetransmitTime                       uint32
	PathMtuDiscoveryTimeout              uint32
	LinkLocalAddressBehavior             uint32
	LinkLocalAddressTimeout              uint32
	ZoneIndices                          [16]uint32
	SitePrefixLength                     uint32
	Metric                               uint32
	NlMtu                                uint32
	Connected                            bool
	SupportsWakeUpPatterns               bool
	SupportsNeighborDiscovery            bool
	SupportsRouterDiscovery              bool
	ReachableTime                        uint32
	TransmitOffload                      uint16
	ReceiveOffload                       uint16
	DisableDefaultRoutes                 bool
}

// This struct is defined in the Go stdlib. However, that definition doesn't go
// up to the CompartmentId, which we need.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa366058(v=vs.85).aspx

type IP_ADAPTER_ADDRESSES struct {
	Length                 uint32
	IfIndex                uint32
	Next                   *IP_ADAPTER_ADDRESSES
	AdapterName            *byte
	FirstUnicastAddress    *windows.IpAdapterUnicastAddress
	FirstAnycastAddress    *windows.IpAdapterAnycastAddress
	FirstMulticastAddress  *windows.IpAdapterMulticastAddress
	FirstDnsServerAddress  *windows.IpAdapterDnsServerAdapter
	DnsSuffix              *uint16
	Description            *uint16
	FriendlyName           *uint16
	PhysicalAddress        [syscall.MAX_ADAPTER_ADDRESS_LENGTH]byte
	PhysicalAddressLength  uint32
	Flags                  uint32
	Mtu                    uint32
	IfType                 uint32
	OperStatus             uint32
	Ipv6IfIndex            uint32
	ZoneIndices            [16]uint32
	FirstPrefix            *windows.IpAdapterPrefix
	TransmitLinkSpeed      uint64
	ReceiveLinkSpeed       uint64
	FirstWinsServerAddress uint64
	FirstGatewayAddress    uint64
	Ipv4Metric             uint32
	Ipv6Metric             uint32
	Luid                   NET_LUID
	Dhcpv4Server           windows.SocketAddress
	CompartmentId          uint32
	/* more fields follow */
}

func (n *NetInterface) ByName(name string) (*net.Interface, error) {
	return net.InterfaceByName(name)
}

func (n *NetInterface) ByIP(ipStr string) (*net.Interface, error) {
	ip := net.ParseIP(ipStr)

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			_, net, err := net.ParseCIDR(addr.String())
			if err != nil {
				return nil, err
			}

			if net.Contains(ip) {
				return &iface, nil
			}
		}
	}
	return nil, &InterfaceForIPNotFoundError{ip: ipStr}
}

func (n *NetInterface) SetMTU(alias string, mtu int) error {
	luid, compartmentID, err := getLuidAndCompartment(alias)
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer func() {
		hcsshim.SetCurrentThreadCompartmentId(0)
		runtime.UnlockOSThread()
	}()
	if err := hcsshim.SetCurrentThreadCompartmentId(compartmentID); err != nil {
		logrus.Error(err)
		return err
	}

	var row MIB_IPINTERFACE_ROW
	row.InterfaceLuid = luid
	row.Family = windows.AF_INET

	r0, _, err := syscall.Syscall(getIpInterfaceEntry.Addr(), 1, uintptr(unsafe.Pointer(&row)), 0, 0)
	if int32(r0) != 0 {
		err := fmt.Errorf("GetIpInterfaceEntry: 0x%x", r0)
		logrus.Error(err)
		return err
	}

	row.NlMtu = uint32(mtu)

	// From https://msdn.microsoft.com/en-us/library/windows/desktop/aa814465(v=vs.85).aspx
	// SitePrefixLength must be 0 for IPv4 interfaces
	row.SitePrefixLength = 0

	r0, _, err = syscall.Syscall(setIpInterfaceEntry.Addr(), 1, uintptr(unsafe.Pointer(&row)), 0, 0)
	if int32(r0) != 0 {
		err := fmt.Errorf("SetIpInterfaceEntry: 0x%x", r0)
		logrus.Error(err)
		return err
	}

	return nil
}

func InterfaceExists(alias string) (bool, error) {
	_, _, err := getLuidAndCompartment(alias)
	if err != nil {
		if _, ok := err.(*InterfaceNotFoundError); ok {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func getLuidAndCompartment(alias string) (NET_LUID, uint32, error) {
	var b []byte
	l := uint32(15000)
	for {
		b = make([]byte, l)
		err := windows.GetAdaptersAddresses(syscall.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX|GAA_FLAG_INCLUDE_ALL_COMPARTMENTS, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])), &l)
		if err == nil {
			if l == 0 {
				return NET_LUID{}, 0, nil
			}
			break
		}
		if err.(syscall.Errno) != syscall.ERROR_BUFFER_OVERFLOW {
			return NET_LUID{}, 0, os.NewSyscallError("getadaptersaddresses", err)
		}
		if l <= uint32(len(b)) {
			return NET_LUID{}, 0, os.NewSyscallError("getadaptersaddresses", err)
		}
	}

	for aa := (*IP_ADAPTER_ADDRESSES)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
		name := syscall.UTF16ToString((*(*[10000]uint16)(unsafe.Pointer(aa.FriendlyName)))[:])
		if name == alias {
			return aa.Luid, aa.CompartmentId, nil
		}
	}

	return NET_LUID{}, 0, &InterfaceNotFoundError{alias: alias}
}
