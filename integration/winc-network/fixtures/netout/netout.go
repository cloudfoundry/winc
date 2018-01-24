package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/miekg/dns"
	"golang.org/x/sys/windows"
)

var (
	iphlpapi        = windows.NewLazySystemDLL("iphlpapi.dll")
	icmpCreateFile  = iphlpapi.NewProc("IcmpCreateFile")
	icmpSendEcho    = iphlpapi.NewProc("IcmpSendEcho")
	icmpCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
)

func main() {
	protocol, addr, port, err := parseArgs()
	if err != nil {
		fatalError(err.Error())
	}

	time.Sleep(5 * time.Second)

	switch protocol {
	case "dns":
		testDNS(addr)
	case "tcp":
		testTCP(addr, port)
	case "udp":
		testUDP(addr, port)
	case "icmp":
		testICMP(addr)
	default:
		fatalError(fmt.Sprintf("invalid protocol: %s", protocol))
	}

}

const (
	IP_REQ_TIMED_OUT = uintptr(11010)
)

type icmp_echo_reply32 struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          *byte
}

func testICMP(addr string) {
	handle, _, err := icmpCreateFile.Call()
	if handle == 0 {
		fmt.Printf("IcmpCreateFile: %s", err.Error())
		os.Exit(1)
	}

	sendSize := uintptr(32)
	sendData := make([]byte, sendSize)

	reply := icmp_echo_reply32{}
	replySize := unsafe.Sizeof(reply) + sendSize + 20
	replyBuffer := make([]byte, replySize)

	ip, err := ipToUint32(addr)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	len, _, err := icmpSendEcho.Call(
		handle,
		uintptr(ip),
		uintptr(unsafe.Pointer(&sendData[0])),
		sendSize,
		uintptr(0),
		uintptr(unsafe.Pointer(&replyBuffer[0])),
		replySize,
		4000,
	)

	defer icmpCloseHandle.Call(handle)
	if len == 0 {
		e := err.(syscall.Errno)
		if uintptr(e) == IP_REQ_TIMED_OUT {
			fmt.Printf("Request timed out\n")
		} else {
			fmt.Printf("IcmpSendEcho: %s\n", err.Error())
		}
		os.Exit(1)
	}

	reply = *((*icmp_echo_reply32)(unsafe.Pointer(&replyBuffer[0])))
	fmt.Printf("recieved %d replies from %s", len, uint32ToIp(reply.Address))
}

func ipToUint32(addr string) (uint32, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return 0, fmt.Errorf("couldn't parse addr: %s", addr)
	}

	ipv4 := ip.To4()
	return uint32(ipv4[3])<<24 + uint32(ipv4[2])<<16 + uint32(ipv4[1])<<8 + uint32(ipv4[0]), nil
}

func uint32ToIp(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip&0xff, (ip>>8)&0xff, (ip>>16)&0xff, ip>>24)
}

func testDNS(host string) {
	addrs, err := net.LookupHost(host)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("found addrs: %+v\n", addrs)
	return
}

func testTCP(addr string, port int) {
	_, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Printf("connected to %s:%d over tcp", addr, port)
}

func testUDP(addr string, port int) {
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
	m := new(dns.Msg)
	m.SetQuestion("google.com.", dns.TypeSOA)

	c := new(dns.Client)
	r, _, err := c.Exchange(m, serverAddr.String())
	if err != nil {
		fmt.Printf("failed to exchange: %v\n", err)
		os.Exit(1)
	}
	if r == nil {
		fmt.Println("response is nil")
		os.Exit(1)
	}
	if r.Rcode != dns.RcodeSuccess {
		fmt.Printf("failed to get an valid answer: %+v\n", r)
		os.Exit(1)
	}

	fmt.Printf("recieved response to DNS query from %s:%d over UDP", addr, port)
}

func parseArgs() (string, string, int, error) {
	var protocol, addr, port string
	flagSet := flag.NewFlagSet("", flag.ContinueOnError)

	flagSet.StringVar(&protocol, "protocol", "", "")
	flagSet.StringVar(&addr, "addr", "", "")
	flagSet.StringVar(&port, "port", "", "")

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		return "", "", 0, err
	}

	if protocol == "" {
		return "", "", 0, fmt.Errorf("missing required flag 'protocol'")
	}

	if addr == "" {
		return "", "", 0, fmt.Errorf("missing required flag 'addr'")
	}

	var parsedPort int

	if port != "" {
		parsedPort, err = strconv.Atoi(port)
		if err != nil {
			return "", "", 0, err
		}
	}

	return protocol, addr, parsedPort, nil
}

func fatalError(e string) {
	fmt.Printf("ERROR: %s", e)
	os.Exit(1)
}
