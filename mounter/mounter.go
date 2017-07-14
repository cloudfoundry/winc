package mounter

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	deleteVolumeMountPointW = kernel32.NewProc("DeleteVolumeMountPointW")
	setVolumeMountPointW    = kernel32.NewProc("SetVolumeMountPointW")
)

type Mounter struct{}

func (m *Mounter) SetPoint(mountPoint, volume string) error {
	if err := setVolumeMountPointW.Find(); err != nil {
		return err
	}

	mountPoint = ensureTrailingBackslash(mountPoint)
	volume = ensureTrailingBackslash(volume)

	mp, err := syscall.UTF16PtrFromString(mountPoint)
	if err != nil {
		return err
	}

	vol, err := syscall.UTF16PtrFromString(volume)
	if err != nil {
		return err
	}

	r0, _, err := syscall.Syscall(setVolumeMountPointW.Addr(), 2, uintptr(unsafe.Pointer(mp)), uintptr(unsafe.Pointer(vol)), 0)
	if int32(r0) == 0 {
		return fmt.Errorf("error setting mount point: %s", err.Error())
	}

	return nil
}

func (m *Mounter) DeletePoint(mountPoint string) error {
	if err := deleteVolumeMountPointW.Find(); err != nil {
		return err
	}

	mountPoint = ensureTrailingBackslash(mountPoint)

	mp, err := syscall.UTF16PtrFromString(mountPoint)
	if err != nil {
		return err
	}

	r0, _, err := syscall.Syscall(deleteVolumeMountPointW.Addr(), 2, uintptr(unsafe.Pointer(mp)), 0, 0)
	if int32(r0) == 0 {
		return fmt.Errorf("error deleting mount point: %s", err.Error())
	}

	return nil
}

func ensureTrailingBackslash(in string) string {
	if !strings.HasSuffix(in, "\\") {
		in += "\\"
	}

	return in
}
