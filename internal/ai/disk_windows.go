//go:build windows

package ai

import (
	"syscall"
	"unsafe"
)

// The space call lives in kernel32, which is part of Windows itself, so it is
// reached through the standard library's lazy loader rather than by taking on
// a dependency for one function.
var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// availableBytes reports the space an unprivileged user can still write on the
// volume holding dir.
//
// GetDiskFreeSpaceExW answers per volume and its first result honours the
// caller's disk quota, which is the closest Windows has to the reserve-aware
// figure the Unix call gives.
func availableBytes(dir string) (uint64, error) {
	if err := getDiskFreeSpaceEx.Find(); err != nil {
		return 0, err
	}
	path, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var freeToCaller, total, free uint64
	ok, _, callErr := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&freeToCaller)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&free)),
	)
	if ok == 0 {
		return 0, callErr
	}
	return freeToCaller, nil
}
