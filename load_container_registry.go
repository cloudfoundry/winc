package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	winio "github.com/Microsoft/go-winio"

	"github.com/Microsoft/hcsshim"
	"golang.org/x/sys/windows"
)

var (
	advapi32         = windows.NewLazySystemDLL("advapi32")
	regLoadKeyW      = advapi32.NewProc("RegLoadKeyW")
	regUnLoadKeyW    = advapi32.NewProc("RegUnLoadKeyW")
	regOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	regCloseKey      = advapi32.NewProc("RegCloseKey")
	regSetKeyValueW  = advapi32.NewProc("RegSetKeyValueW")
	regQueryValueExW = advapi32.NewProc("RegQueryValueExW")
)

const (
	HKEY_LOCAL_MACHINE = uintptr(0x80000002)
	KEY_ALL_ACCESS     = uintptr(0xF003F)
	REG_BINARY         = uintptr(0x3)
)

func main() {
	id := os.Args[1]
	pid, err := containerPid(id)
	if err != nil {
		panic(err)
	}

	if err := winio.EnableProcessPrivileges([]string{"SeBackupPrivilege", "SeRestorePrivilege"}); err != nil {
		panic(err)
	}
	defer winio.DisableProcessPrivileges([]string{"SeBackupPrivilege", "SeRestorePrivilege"})

	hive := filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "Windows", "System32", "Config", "SYSTEM")
	h, err := syscall.UTF16PtrFromString(hive)
	if err != nil {
		panic(err)
	}

	keyName, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		panic(err)
	}

	r0, _, _ := regLoadKeyW.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(keyName)),
		uintptr(unsafe.Pointer(h)),
	)

	if r0 != 0 {
		fmt.Printf("RegLoadKeyW: %s\n", windowsErrorMessage(uint32(r0)))
		return
	}

	defer func() {
		r0, _, _ := regUnLoadKeyW.Call(
			HKEY_LOCAL_MACHINE,
			uintptr(unsafe.Pointer(keyName)),
		)

		if r0 != 0 {
			fmt.Printf("RegUnLoadKeyW: %s\n", windowsErrorMessage(uint32(r0)))
			return
		}
	}()

	//	subKey := filepath.Join(id, "ControlSet001", "Services", "HTTP", "Parameters", "UrlAclInfo")
	//	sk, err := syscall.UTF16PtrFromString(subKey)
	//	if err != nil {
	//		panic(err)
	//	}
	//
	//	var aclKey syscall.Handle
	//	r0, _, _ = regOpenKeyExW.Call(
	//		HKEY_LOCAL_MACHINE,
	//		uintptr(unsafe.Pointer(sk)),
	//		uintptr(0),
	//		KEY_ALL_ACCESS,
	//		uintptr(unsafe.Pointer(&aclKey)),
	//	)
	//	if r0 != 0 {
	//		fmt.Printf("RegOpenKeyExW: %s\n", windowsErrorMessage(uint32(r0)))
	//		return
	//	}
	//
	//	url := "http://sams-cool-website:5566"
	//	u, err := syscall.UTF16PtrFromString(url)
	//	if err != nil {
	//		panic(err)
	//	}
	//
	//	data := []byte{0xd, 0xe, 0xa, 0xd, 0xb, 0xe, 0xe, 0xf}
	//
	//	r0, _, _ = regSetKeyValueW.Call(
	//		uintptr(aclKey),
	//		uintptr(0),
	//		uintptr(unsafe.Pointer(u)),
	//		REG_BINARY,
	//		uintptr(unsafe.Pointer(&data[0])),
	//		uintptr(8),
	//	)
	//	if r0 != 0 {
	//		fmt.Printf("RegSetKeyValueW: %s\n", windowsErrorMessage(uint32(r0)))
	//		return
	//	}

}

func getContainerControlSet(containerId string) (uint32, error) {
	selectKey, err := openKey(filepath.Join(containerId, "Select"))
	if err != nil {
		return 0, err
	}
	defer closeKey(selectKey)

	var currentControlSet uint32
	dataSize := unsafe.Sizeof(currentControlSet)
	name, err := syscall.UTF16PtrFromString("Current")
	if err != nil {
		return 0, err
	}

	r0, _, _ := regQueryValueExW.Call(
		uintptr(selectKey),
		uintptr(unsafe.Pointer(name)),
		uintptr(0),
		uintptr(0),
		uintptr(unsafe.Pointer(&currentControlSet)),
		uintptr(unsafe.Pointer(&dataSize)),
	)

	if r0 != 0 {
		return 0, fmt.Errorf("RegQueryValueExW: %s\n", windowsErrorMessage(uint32(r0)))
	}

	return currentControlSet, nil
}

func openKey(key string) (syscall.Handle, error) {
	k, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return 0, err
	}

	var h syscall.Handle
	r0, _, _ := regOpenKeyExW.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(k)),
		uintptr(0),
		KEY_ALL_ACCESS,
		uintptr(unsafe.Pointer(&h)),
	)

	if r0 != 0 {
		return 0, fmt.Errorf("RegOpenKeyExW: %s\n", windowsErrorMessage(uint32(r0)))
	}

	return h, nil
}

func closeKey(h syscall.Handle) error {
	r0, _, _ := regCloseKey.Call(uintptr(h))
	if r0 != 0 {
		return fmt.Errorf("RegCloseKeyW: %s\n", windowsErrorMessage(uint32(r0)))
	}
	return nil
}

func containerPid(id string) (int, error) {
	container, err := hcsshim.OpenContainer(id)
	if err != nil {
		return -1, err
	}

	pl, err := container.ProcessList()
	if err != nil {
		return -1, err
	}

	var process hcsshim.ProcessListItem
	oldestTime := time.Now()
	for _, v := range pl {
		if v.ImageName == "wininit.exe" && v.CreateTimestamp.Before(oldestTime) {
			oldestTime = v.CreateTimestamp
			process = v
		}
	}

	return int(process.ProcessId), nil
}

func windowsErrorMessage(code uint32) string {
	flags := uint32(windows.FORMAT_MESSAGE_FROM_SYSTEM | windows.FORMAT_MESSAGE_IGNORE_INSERTS)
	langId := uint32(windows.SUBLANG_ENGLISH_US)<<10 | uint32(windows.LANG_ENGLISH)
	buf := make([]uint16, 512)

	_, err := windows.FormatMessage(flags, uintptr(0), code, langId, buf, nil)
	if err != nil {
		return fmt.Sprintf("0x%x", code)
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}
