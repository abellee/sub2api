//go:build !linux && !windows

package relay

import "fmt"

func diskUsedPercent(string) (float64, error) {
	return 0, fmt.Errorf("disk pressure cleanup is unsupported on this platform")
}
