package volume

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

type Statser struct{}

func (s *Statser) getDiskFreespace(volumePath string) (uint64, uint64, error) {
	if err := getDiskFreeSpaceExW.Find(); err != nil {
		return 0, 0, fmt.Errorf("error loading dll: %s", err.Error())
	}

	volumePath = ensureTrailingBackslash(volumePath)
	vol, err := syscall.UTF16PtrFromString(volumePath)
	if err != nil {
		return 0, 0, fmt.Errorf("error converting %s to utf16 pointer", volumePath)
	}

	var freeBytes, totalBytes, totalFreeBytes uint64
	r0, _, err := syscall.Syscall6(getDiskFreeSpaceExW.Addr(), 4,
		uintptr(unsafe.Pointer(vol)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)), 0, 0)

	if int32(r0) == 0 {
		return 0, 0, fmt.Errorf("error getting volume freespace: %s\n", err.Error())
	}

	return freeBytes, totalBytes, nil
}

func (s *Statser) GetCurrentDiskUsage(volumePath string) (uint64, error) {
	freeBytes, totalBytes, err := s.getDiskFreespace(volumePath)
	if err != nil {
		return 0, err
	}

	return totalBytes - freeBytes, nil
}
