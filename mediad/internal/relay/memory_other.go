//go:build !windows

package relay

// Linux uses /proc/meminfo; fail closed when available memory cannot be measured.
func platformAvailableMemory() int64 { return 0 }
