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

type AdapterInfo struct {
	Name            string
	Index           uint32
	LUID            NET_LUID
	PhysicalAddress net.HardwareAddr
	CompartmentId   uint32
}

type InterfaceNotFoundError struct {
	name  string
	index uint32
}

func (e *InterfaceNotFoundError) Error() string {
	return fmt.Sprintf("interface with name: %s, index: %d not found", e.name, e.index)
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
	PhysicalAddress        [windows.MAX_ADAPTER_ADDRESS_LENGTH]byte
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

func (n *NetInterface) ByName(name string) (AdapterInfo, error) {
	return getAdapterInfoByName(name, windows.AF_INET)
}

func (n *NetInterface) ByIP(ipStr string) (AdapterInfo, error) {
	ip := net.ParseIP(ipStr)

	ifaces, err := net.Interfaces()
	if err != nil {
		return AdapterInfo{}, err
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return AdapterInfo{}, err
		}

		for _, addr := range addrs {
			_, net, err := net.ParseCIDR(addr.String())
			if err != nil {
				return AdapterInfo{}, err
			}

			if net.Contains(ip) {
				return getAdapterInfoByIndex(uint32(iface.Index), windows.AF_INET)
			}
		}
	}
	return AdapterInfo{}, &InterfaceForIPNotFoundError{ip: ipStr}
}

func (n *NetInterface) SetMTU(name string, mtu uint32, family uint32) error {
	adapterInfo, err := getAdapterInfoByName(name, family)
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer func() {
		hcsshim.SetCurrentThreadCompartmentId(0)
		runtime.UnlockOSThread()
	}()
	if err := hcsshim.SetCurrentThreadCompartmentId(adapterInfo.CompartmentId); err != nil {
		logrus.Error(err)
		return err
	}

	var row MIB_IPINTERFACE_ROW
	row.InterfaceLuid = adapterInfo.LUID
	row.Family = family

	r0, _, err := syscall.Syscall(getIpInterfaceEntry.Addr(), 1, uintptr(unsafe.Pointer(&row)), 0, 0)
	if int32(r0) != 0 {
		err := fmt.Errorf("GetIpInterfaceEntry: 0x%x", r0)
		logrus.Error(err)
		return err
	}

	row.NlMtu = mtu

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

func (n *NetInterface) GetMTU(name string, family uint32) (uint32, error) {
	adapterInfo, err := getAdapterInfoByName(name, family)
	if err != nil {
		return 0, err
	}

	runtime.LockOSThread()
	defer func() {
		hcsshim.SetCurrentThreadCompartmentId(0)
		runtime.UnlockOSThread()
	}()
	if err := hcsshim.SetCurrentThreadCompartmentId(adapterInfo.CompartmentId); err != nil {
		logrus.Error(err)
		return 0, err
	}

	var row MIB_IPINTERFACE_ROW
	row.InterfaceLuid = adapterInfo.LUID
	row.Family = family

	r0, _, err := syscall.Syscall(getIpInterfaceEntry.Addr(), 1, uintptr(unsafe.Pointer(&row)), 0, 0)
	if int32(r0) != 0 {
		err := fmt.Errorf("GetIpInterfaceEntry: 0x%x", r0)
		logrus.Error(err)
		return 0, err
	}

	return row.NlMtu, nil
}

func InterfaceExists(name string) (bool, error) {
	_, err := getAdapterInfoByName(name, windows.AF_UNSPEC)
	if err != nil {
		if _, ok := err.(*InterfaceNotFoundError); ok {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func getAdapterInfoByName(name string, family uint32) (AdapterInfo, error) {
	var b []byte
	l := uint32(15000)
	for {
		b = make([]byte, l)
		err := windows.GetAdaptersAddresses(family, windows.GAA_FLAG_INCLUDE_PREFIX|GAA_FLAG_INCLUDE_ALL_COMPARTMENTS, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])), &l)
		if err == nil {
			if l == 0 {
				return AdapterInfo{}, nil
			}
			break
		}
		if err.(syscall.Errno) != syscall.ERROR_BUFFER_OVERFLOW {
			return AdapterInfo{}, os.NewSyscallError("getadaptersaddresses", err)
		}
		if l <= uint32(len(b)) {
			return AdapterInfo{}, os.NewSyscallError("getadaptersaddresses", err)
		}
	}

	for aa := (*IP_ADAPTER_ADDRESSES)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
		foundName := syscall.UTF16ToString((*(*[10000]uint16)(unsafe.Pointer(aa.FriendlyName)))[:])
		var physicalAddress net.HardwareAddr
		if aa.PhysicalAddressLength > 0 {
			physicalAddress = make(net.HardwareAddr, aa.PhysicalAddressLength)
			copy(physicalAddress, aa.PhysicalAddress[:])
		}
		if foundName == name {
			return AdapterInfo{
				Name:            foundName,
				Index:           aa.IfIndex,
				LUID:            aa.Luid,
				CompartmentId:   aa.CompartmentId,
				PhysicalAddress: physicalAddress,
			}, nil
		}
	}

	return AdapterInfo{}, &InterfaceNotFoundError{name: name}
}

func getAdapterInfoByIndex(ifIdx uint32, family uint32) (AdapterInfo, error) {
	var b []byte
	l := uint32(15000)
	for {
		b = make([]byte, l)
		err := windows.GetAdaptersAddresses(family, windows.GAA_FLAG_INCLUDE_PREFIX|GAA_FLAG_INCLUDE_ALL_COMPARTMENTS, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&b[0])), &l)
		if err == nil {
			if l == 0 {
				return AdapterInfo{}, nil
			}
			break
		}
		if err.(syscall.Errno) != syscall.ERROR_BUFFER_OVERFLOW {
			return AdapterInfo{}, os.NewSyscallError("getadaptersaddresses", err)
		}
		if l <= uint32(len(b)) {
			return AdapterInfo{}, os.NewSyscallError("getadaptersaddresses", err)
		}
	}

	for aa := (*IP_ADAPTER_ADDRESSES)(unsafe.Pointer(&b[0])); aa != nil; aa = aa.Next {
		if aa.IfIndex == ifIdx {
			name := syscall.UTF16ToString((*(*[10000]uint16)(unsafe.Pointer(aa.FriendlyName)))[:])
			var physicalAddress net.HardwareAddr
			if aa.PhysicalAddressLength > 0 {
				physicalAddress = make(net.HardwareAddr, aa.PhysicalAddressLength)
				copy(physicalAddress, aa.PhysicalAddress[:])
			}
			return AdapterInfo{
				Name:            name,
				Index:           aa.IfIndex,
				LUID:            aa.Luid,
				CompartmentId:   aa.CompartmentId,
				PhysicalAddress: physicalAddress,
			}, nil
		}
	}

	return AdapterInfo{}, &InterfaceNotFoundError{index: ifIdx}
}
