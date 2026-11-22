package iqdetectd

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseHHMM parses a "HH:MM" clock time into minutes-of-day.
func parseHHMM(value string) (int, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	hours, errH := strconv.Atoi(parts[0])
	minutes, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil ||
		hours < 0 || hours > 23 || minutes < 0 || minutes > 59 ||
		len(parts[0]) > 2 || len(parts[1]) != 2 {
		return 0, false
	}
	return hours*60 + minutes, true
}

// normalizeQuietWindows validates the submitted quiet windows: both bounds
// must be "HH:MM"; incomplete rows are dropped, malformed rows reject the
// whole payload. Start == End is treated as a full-day block.
func normalizeQuietWindows(windows []QuietWindow) ([]QuietWindow, string) {
	normalized := make([]QuietWindow, 0, len(windows))
	for _, w := range windows {
		start, end := strings.TrimSpace(w.Start), strings.TrimSpace(w.End)
		if start == "" && end == "" {
			continue
		}
		if _, ok := parseHHMM(start); !ok {
			return nil, fmt.Sprintf("invalid quiet window start %q, expected HH:MM", start)
		}
		if _, ok := parseHHMM(end); !ok {
			return nil, fmt.Sprintf("invalid quiet window end %q, expected HH:MM", end)
		}
		normalized = append(normalized, QuietWindow{Start: start, End: end})
	}
	return normalized, ""
}

// quietWindowZone 是不检测时间段的时区:北京时间(UTC+8,无夏令时),
// 与边车部署机器的本地时区无关。
var quietWindowZone = time.FixedZone("UTC+8", 8*60*60)

// inQuietWindows reports whether t falls inside any quiet window, with the
// window bounds interpreted as Beijing time (UTC+8). A window whose start is
// after its end wraps midnight.
func inQuietWindows(windows []QuietWindow, t time.Time) bool {
	local := t.In(quietWindowZone)
	now := local.Hour()*60 + local.Minute()
	for _, w := range windows {
		start, okStart := parseHHMM(w.Start)
		end, okEnd := parseHHMM(w.End)
		if !okStart || !okEnd {
			continue
		}
		if start == end {
			return true
		}
		if start < end {
			if now >= start && now < end {
				return true
			}
		} else if now >= start || now < end { // 跨天：23:00-07:00
			return true
		}
	}
	return false
}
