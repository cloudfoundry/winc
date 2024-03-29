package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var (
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	deleteVolumeMountPointW = kernel32.NewProc("DeleteVolumeMountPointW")
	setVolumeMountPointW    = kernel32.NewProc("SetVolumeMountPointW")
)

type Mounter struct{}

func (m *Mounter) Mount(pid int, volumePath string, logger *logrus.Entry) error {
	if _, err := os.Stat(mountPath(pid)); !os.IsNotExist(err) {
		err := fmt.Errorf("mountdir exists: %s", mountPath(pid))
		logger.Error(err.Error())
		return err
	}

	if err := os.MkdirAll(rootPath(pid), 0755); err != nil {
		return err
	}

	return m.setPoint(rootPath(pid), volumePath)
}

func (m *Mounter) Unmount(pid int) error {
	defer os.RemoveAll(mountPath(pid))
	if err := m.deletePoint(rootPath(pid)); err != nil {
		return err
	}

	return os.RemoveAll(mountPath(pid))
}

func (m *Mounter) setPoint(mountPoint, volume string) error {
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

	r0, _, err := syscall.SyscallN(setVolumeMountPointW.Addr(), uintptr(unsafe.Pointer(mp)), uintptr(unsafe.Pointer(vol)), 0)
	if int32(r0) == 0 {
		return fmt.Errorf("error setting mount point: %s", err.Error())
	}

	return nil
}

func (m *Mounter) deletePoint(mountPoint string) error {
	if err := deleteVolumeMountPointW.Find(); err != nil {
		return err
	}

	mountPoint = ensureTrailingBackslash(mountPoint)

	mp, err := syscall.UTF16PtrFromString(mountPoint)
	if err != nil {
		return err
	}

	r0, _, err := syscall.SyscallN(deleteVolumeMountPointW.Addr(), uintptr(unsafe.Pointer(mp)), 0, 0)
	if int32(r0) == 0 {
		return fmt.Errorf("error deleting mount point: %s", err.Error())
	}

	return nil
}

func mountPath(pid int) string {
	return filepath.Join("c:\\", "proc", strconv.Itoa(pid))
}

func rootPath(pid int) string {
	return filepath.Join(mountPath(pid), "root")
}

func ensureTrailingBackslash(in string) string {
	if !strings.HasSuffix(in, "\\") {
		in += "\\"
	}

	return in
}
