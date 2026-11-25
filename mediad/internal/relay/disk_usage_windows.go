package relay

import (
	"fmt"
	"syscall"
	"unsafe"
)

func diskUsedPercent(path string) (float64, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available, total, free uint64
	ok, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW").Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
	if ok == 0 {
		return 0, err
	}
	if total == 0 || available > total {
		return 0, fmt.Errorf("invalid filesystem capacity")
	}
	return 100 * (1 - float64(available)/float64(total)), nil
}
