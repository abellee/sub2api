package relay

import (
	"fmt"
	"syscall"
)

func diskUsedPercent(path string) (float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	if stat.Blocks == 0 || stat.Bavail > stat.Blocks {
		return 0, fmt.Errorf("invalid filesystem capacity")
	}
	return 100 * (1 - float64(stat.Bavail)/float64(stat.Blocks)), nil
}
