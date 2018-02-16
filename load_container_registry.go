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
	advapi32       = windows.NewLazySystemDLL("advapi32")
	regLoadKeyW    = advapi32.NewProc("RegLoadKeyW")
	regUnLoadKeyW  = advapi32.NewProc("RegUnLoadKeyW")
	regOpenKeyW    = advapi32.NewProc("RegOpenKeyW")
	regCloseKey    = advapi32.NewProc("RegCloseKey")
	regSetValueW   = advapi32.NewProc("RegSetValueW")
	regQueryValueW = advapi32.NewProc("RegQueryValueW")
)

const (
	HKEY_LOCAL_MACHINE = uintptr(0x80000002)
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
			uintptr(HKEY_LOCAL_MACHINE),
			uintptr(unsafe.Pointer(keyName)),
		)

		if r0 != 0 {
			fmt.Printf("RegUnLoadKeyW: %s\n", windowsErrorMessage(uint32(r0)))
			return
		}
	}()

	subKey := filepath.Join(id, "ControlSet001", "Services", "HTTP", "Parameters", "UrlAclInfo")
	sk, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		panic(err)
	}

	var aclKey syscall.Handle
	r0, _, _ = regOpenKeyW.Call(
		uintptr(HKEY_LOCAL_MACHINE),
		uintptr(unsafe.Pointer(sk)),
		uintptr(unsafe.Pointer(&aclKey)),
	)
	if r0 != 0 {
		fmt.Printf("RegOpenKeyW: %s\n", windowsErrorMessage(uint32(r0)))
		return
	}

	defer func() {
		r0, _, _ := regCloseKey.Call(
			uintptr(aclKey),
		)

		if r0 != 0 {
			fmt.Printf("RegCloseKeyW: %s\n", windowsErrorMessage(uint32(r0)))
			return
		}
	}()

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
