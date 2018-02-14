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
	orEnumValue    = offreg.NewProc("OREnumValue")
	orSetValue     = offreg.NewProc("ORSetValue")
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
	var numValues uint32
	var maxValueNameLen uint32
	var maxValueDataLen uint32

	path := `ControlSet001\Services\HTTP\Parameters\UrlAclInfo`
	handle, err := openKey(root, path)
	if err != nil {
		return err
	}
	defer closeKey(handle)

	valueName := "http://sams.cool.website:4444/"
	vn, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return nil
	}

	data := []byte{0xd, 0xe, 0xa, 0xd, 0xb, 0xe, 0xe, 0xf}

	r0, _, _ := orSetValue.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(vn)),
		uintptr(windows.REG_BINARY),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(8),
	)

	if r0 != 0 {
		return fmt.Errorf("ORSetKey: %s\n", windowsErrorMessage(uint32(r0)))
	}

	r0, _, _ = orQueryInfoKey.Call(
		uintptr(handle),
		uintptr(0),
		uintptr(0),
		uintptr(0),                                //		  _Out_opt_   PDWORD    lpcSubKeys,
		uintptr(0),                                //  _Out_opt_   PDWORD    lpcMaxSubKeyLen,
		uintptr(0),                                //  _Out_opt_   PDWORD    lpcMaxClassLen,
		uintptr(unsafe.Pointer(&numValues)),       //  _Out_opt_   PDWORD    lpcValues,
		uintptr(unsafe.Pointer(&maxValueNameLen)), //  _Out_opt_   PDWORD    lpcMaxValueNameLen,
		uintptr(unsafe.Pointer(&maxValueDataLen)), //  _Out_opt_   PDWORD    lpcMaxValueLen,
		uintptr(0), //  _Out_opt_   PDWORD    lpcbSecurityDescriptor,
		uintptr(0), //  _Out_opt_   PFILETIME lpftLastWriteTime
	)

	if r0 != 0 {
		return fmt.Errorf("ORQueryInfoKey: %s\n", windowsErrorMessage(uint32(r0)))
	}

	fmt.Println(numValues)
	fmt.Println(maxValueNameLen)
	fmt.Println(maxValueDataLen)
	valueNameBuf := make([]uint16, maxValueNameLen+1)

	for i := uint32(0); i < numValues; i++ {
		size := maxValueNameLen + 1

		r0, _, _ = orEnumValue.Call(
			uintptr(handle),
			uintptr(i),
			uintptr(unsafe.Pointer(&valueNameBuf[0])),
			uintptr(unsafe.Pointer(&size)),
			uintptr(0),
			uintptr(0),
			uintptr(0),
		)

		if r0 != 0 {
			return fmt.Errorf("OREnumValue: %s\n", windowsErrorMessage(uint32(r0)))
		}

		fmt.Println(syscall.UTF16ToString(valueNameBuf))
	}

	return nil
}

func openKey(handle syscall.Handle, subKeyName string) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(subKeyName)
	if err != nil {
		return 0, err
	}

	var key syscall.Handle
	r0, _, _ := orOpenKey.Call(
		uintptr(unsafe.Pointer(handle)),
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&key)),
	)
	if r0 != 0 {
		return 0, fmt.Errorf("OROpenKey: %s\n", windowsErrorMessage(uint32(r0)))
	}

	return key, nil
}

func closeKey(handle syscall.Handle) error {
	r0, _, _ := orCloseKey.Call(
		uintptr(unsafe.Pointer(handle)),
	)
	if r0 != 0 {
		return fmt.Errorf("ORCloseKey: %s\n", windowsErrorMessage(uint32(r0)))
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
