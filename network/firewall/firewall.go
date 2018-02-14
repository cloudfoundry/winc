package firewall

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Firewall struct {
	dll        *windows.DLL
	deleteRule *windows.Proc
	createRule *windows.Proc
	ruleExists *windows.Proc
}

// consts taken from here: https://msdn.microsoft.com/en-us/library/windows/desktop/aa366327(v=vs.85).aspx
type Action int

const (
	NET_FW_ACTION_BLOCK Action = 0
	NET_FW_ACTION_ALLOW Action = 1
)

type Direction int

const (
	NET_FW_RULE_DIR_IN  Direction = 1
	NET_FW_RULE_DIR_OUT Direction = 2
)

type Protocol int

const (
	NET_FW_IP_PROTOCOL_ICMP Protocol = 1
	NET_FW_IP_PROTOCOL_TCP  Protocol = 6
	NET_FW_IP_PROTOCOL_UDP  Protocol = 17
	NET_FW_IP_PROTOCOL_ANY  Protocol = 256
)

type Rule struct {
	Name            string
	Action          Action
	Direction       Direction
	Protocol        Protocol
	LocalAddresses  string
	LocalPorts      string
	RemoteAddresses string
	RemotePorts     string
}

func (f *Firewall) CreateRule(rule Rule) error {
	name, err := syscall.UTF16PtrFromString(rule.Name)
	if err != nil {
		return err
	}

	localAddresses, err := syscall.UTF16PtrFromString(rule.LocalAddresses)
	if err != nil {
		return err
	}

	localPorts, err := syscall.UTF16PtrFromString(rule.LocalPorts)
	if err != nil {
		return err
	}

	remoteAddresses, err := syscall.UTF16PtrFromString(rule.RemoteAddresses)
	if err != nil {
		return err
	}

	remotePorts, err := syscall.UTF16PtrFromString(rule.RemotePorts)
	if err != nil {
		return err
	}

	r0, _, err := f.createRule.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(rule.Action),
		uintptr(rule.Direction),
		uintptr(rule.Protocol),
		uintptr(unsafe.Pointer(localAddresses)),
		uintptr(unsafe.Pointer(localPorts)),
		uintptr(unsafe.Pointer(remoteAddresses)),
		uintptr(unsafe.Pointer(remotePorts)),
	)

	if int32(r0) != 0 {
		return fmt.Errorf("error creating rule: %s\n", err.Error())
	}

	return nil
}

func (f *Firewall) DeleteRule(name string) error {
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}

	r0, _, err := f.deleteRule.Call(uintptr(unsafe.Pointer(n)))
	if int32(r0) != 0 {
		return fmt.Errorf("error deleting rule: %s\n", err.Error())
	}

	return nil
}

func (f *Firewall) RuleExists(name string) (bool, error) {
	n, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false, err
	}

	r0, _, err := f.ruleExists.Call(uintptr(unsafe.Pointer(n)))
	if int32(r0) == -1 {
		return false, fmt.Errorf("error checking rule exists: %s\n", err.Error())
	}

	return r0 == 1, nil
}

func (f *Firewall) Close() error {
	return f.dll.Release()
}

func NewFirewall(firewallDLL string) (*Firewall, error) {
	var err error
	exeFile := ""

	if firewallDLL == "" {
		exeFile, err = os.Executable()
		if err != nil {
			return nil, err
		}
		exeDir := filepath.Dir(exeFile)
		firewallDLL = filepath.Join(exeDir, "firewall.dll")
	}

	firewall, err := windows.LoadDLL(firewallDLL)
	if err != nil {
		return nil, err
	}

	createRule, err := firewall.FindProc("CreateRule")
	if err != nil {
		return nil, err
	}
	deleteRule, err := firewall.FindProc("DeleteRule")
	if err != nil {
		return nil, err
	}
	ruleExists, err := firewall.FindProc("RuleExists")
	if err != nil {
		return nil, err
	}

	return &Firewall{
		dll:        firewall,
		createRule: createRule,
		deleteRule: deleteRule,
		ruleExists: ruleExists,
	}, nil
}
