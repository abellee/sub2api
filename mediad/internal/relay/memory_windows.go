package relay

import (
	"syscall"
	"unsafe"
)

func platformAvailableMemory() int64 {
	var status struct {
		Length, Load                                                                          uint32
		TotalPhys, AvailPhys, TotalPage, AvailPage, TotalVirtual, AvailVirtual, AvailExtended uint64
	}
	status.Length = uint32(unsafe.Sizeof(status))
	ok, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return 0
	}
	return int64(status.AvailPhys)
}
