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

	"github.com/Microsoft/hcsshim"
	"golang.org/x/sys/windows"
)

var (
	offreg         = windows.NewLazySystemDLL("offreg.dll")
	orOpenHive     = offreg.NewProc("OROpenHive")
	orCloseHive    = offreg.NewProc("ORCloseHive")
	orOpenKey      = offreg.NewProc("OROpenKey")
	orQueryInfoKey = offreg.NewProc("ORQueryInfoKey")
	orEnumKey      = offreg.NewProc("OREnumKey")
	orCloseKey     = offreg.NewProc("ORCloseKey")
)

const (
	KEY_ALL_ACCESS     = 0xF003F
	REG_PROCESS_APPKEY = 0x1
)

func main() {
	id := os.Args[1]
	pid, err := containerPid(id)
	if err != nil {
		panic(err)
	}

	hive := filepath.Join("c:\\", "proc", strconv.Itoa(pid), "root", "Windows", "System32", "Config", "SYSTEM")
	h, err := syscall.UTF16PtrFromString(hive)
	if err != nil {
		panic(err)
	}

	var root syscall.Handle
	r0, _, _ := orOpenHive.Call(
		uintptr(unsafe.Pointer(h)),
		uintptr(unsafe.Pointer(&root)),
	)

	if r0 != 0 {
		fmt.Printf("OROpenHive: %s\n", windowsErrorMessage(uint32(r0)))
		return
	}

	defer orCloseHive.Call(uintptr(root))

	if err := doRegistryStuff(root); err != nil {
		fmt.Println(err.Error())
	}

}

func doRegistryStuff(root syscall.Handle) error {
	var numSubKeys uint32
	var maxSubKeyLen uint32

	r0, _, _ := orQueryInfoKey.Call(
		uintptr(root),
		uintptr(0),
		uintptr(0),
		uintptr(unsafe.Pointer(&numSubKeys)),   //		  _Out_opt_   PDWORD    lpcSubKeys,
		uintptr(unsafe.Pointer(&maxSubKeyLen)), //  _Out_opt_   PDWORD    lpcMaxSubKeyLen,
		uintptr(0),                             //  _Out_opt_   PDWORD    lpcMaxClassLen,
		uintptr(0),                             //  _Out_opt_   PDWORD    lpcValues,
		uintptr(0),                             //  _Out_opt_   PDWORD    lpcMaxValueNameLen,
		uintptr(0),                             //  _Out_opt_   PDWORD    lpcMaxValueLen,
		uintptr(0),                             //  _Out_opt_   PDWORD    lpcbSecurityDescriptor,
		uintptr(0),                             //  _Out_opt_   PFILETIME lpftLastWriteTime
	)

	if r0 != 0 {
		return fmt.Errorf("ORQueryInfoKey: %s\n", windowsErrorMessage(uint32(r0)))
	}

	fmt.Println(numSubKeys)
	fmt.Println(maxSubKeyLen)

	foo := uint32(10000)

	for i := uint32(0); i < numSubKeys; i++ {
		keyNameBuf := make([]uint16, 10000)

		r0, _, _ = orEnumKey.Call(
			uintptr(root),
			uintptr(i),
			uintptr(unsafe.Pointer(&keyNameBuf[0])),
			uintptr(unsafe.Pointer(&foo)),
			uintptr(0),
			uintptr(0),
			uintptr(0),
		)

		if r0 != 0 {
			return fmt.Errorf("OREnumKey: %s\n", windowsErrorMessage(uint32(r0)))
		}

		fmt.Println(syscall.UTF16ToString(keyNameBuf))

	}

	//pathKey := `HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	//p, err := syscall.UTF16PtrFromString(pathKey)
	//if err != nil {
	//	panic(err)
	//}

	//	var env syscall.Handle
	//	r0, _, _ = orOpenKey.Call(
	//		uintptr(unsafe.Pointer(root)),
	//		uintptr(unsafe.Pointer(p)),
	//		uintptr(unsafe.Pointer(&env)),
	//	)
	//	if r0 != 0 {
	//		fmt.Printf("OROpenKey: %s\n", windowsErrorMessage(uint32(r0)))
	//		return
	//	}
	//
	//	r0, _, _ = orCloseKey.Call(
	//		uintptr(unsafe.Pointer(env)),
	//	)
	//	if r0 != 0 {
	//		fmt.Printf("ORCloseKey: %s\n", windowsErrorMessage(uint32(r0)))
	//		return
	//	}

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
