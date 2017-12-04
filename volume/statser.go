package volume

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

type Statser struct{}

func (s *Statser) GetCurrentDiskUsage(volumePath string) (uint64, error) {
	exeFile, err := os.Executable()
	if err != nil {
		return 0, err
	}

	exeDir := filepath.Dir(exeFile)
	quota := windows.NewLazySystemDLL(filepath.Join(exeDir, "quota.dll"))
	setQuota := quota.NewProc("GetQuotaUsed")

	volume, err := syscall.UTF16PtrFromString(volumePath)
	if err != nil {
		return 0, err
	}
	var quotaUsed uint64

	r0, _, err := syscall.Syscall(setQuota.Addr(), 2, uintptr(unsafe.Pointer(volume)), uintptr(unsafe.Pointer(&quotaUsed)), 0)
	if int32(r0) != 0 {
		return 0, fmt.Errorf("error getting quota: %s\n", err.Error())
	}

	return quotaUsed, nil
}
