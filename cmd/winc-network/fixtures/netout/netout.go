package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/miekg/dns"
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

func testICMP(addr string) {
	cmd := exec.Command("cmd.exe", "/c", fmt.Sprintf("ping %s -n 4", addr))
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		os.Exit(1)
	}
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
