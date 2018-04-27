package winsyscall

import (
	"syscall"
)

type WinSyscall struct{}

func (w *WinSyscall) OpenProcess(flags uint32, inherit bool, pid uint32) (syscall.Handle, error) {
	return syscall.OpenProcess(flags, inherit, pid)
}

func (w *WinSyscall) GetProcessStartTime(handle syscall.Handle) (syscall.Filetime, error) {
	var (
		creationTime syscall.Filetime
		exitTime     syscall.Filetime
		kernelTime   syscall.Filetime
		userTime     syscall.Filetime
	)

	if err := syscall.GetProcessTimes(handle, &creationTime, &exitTime, &kernelTime, &userTime); err != nil {
		return syscall.Filetime{}, err
	}
	return creationTime, nil
}

func (w *WinSyscall) CloseHandle(handle syscall.Handle) error {
	return syscall.CloseHandle(handle)
}

func (w *WinSyscall) GetExitCodeProcess(handle syscall.Handle) (uint32, error) {
	var exitCode uint32
	err := syscall.GetExitCodeProcess(handle, &exitCode)
	return exitCode, err
}
