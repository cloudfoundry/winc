package volume

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const MINIMUM_DISK_QUOTA = 1500

type Limiter struct{}

func (l *Limiter) SetDiskLimit(volumePath string, size uint64) error {
	if size == 0 {
		return nil
	}

	if size < MINIMUM_DISK_QUOTA {
		return fmt.Errorf("requested disk quota %d less than minumum (%d bytes)", size, MINIMUM_DISK_QUOTA)
	}

	exeFile, err := os.Executable()
	if err != nil {
		return err
	}

	exeDir := filepath.Dir(exeFile)
	quota := windows.NewLazySystemDLL(filepath.Join(exeDir, "quota.dll"))
	setQuota := quota.NewProc("SetQuota")

	volume, err := syscall.UTF16PtrFromString(volumePath)
	if err != nil {
		return err
	}

	r0, _, err := syscall.Syscall(setQuota.Addr(), 2, uintptr(unsafe.Pointer(volume)), uintptr(size), 0)
	if int32(r0) != 0 {
		return fmt.Errorf("error setting quota: %s\n", err.Error())
	}

	return nil
}
