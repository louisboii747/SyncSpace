//go:build windows

package transfer

import (
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func availableStorage(path string) int64 {
	absolute, err := filepath.Abs(storageProbePath(path))
	if err != nil {
		return -1
	}
	root := filepath.VolumeName(absolute) + `\`
	pointer, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return -1
	}
	var available uint64
	procedure := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
	result, _, _ := procedure.Call(uintptr(unsafe.Pointer(pointer)), uintptr(unsafe.Pointer(&available)), 0, 0)
	if result == 0 {
		return -1
	}
	if available > uint64(^uint64(0)>>1) {
		return int64(^uint64(0) >> 1)
	}
	return int64(available)
}
