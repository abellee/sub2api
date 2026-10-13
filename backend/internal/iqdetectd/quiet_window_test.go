package iqdetectd

import (
	"context"
	"testing"
	"time"
)

func TestParseHHMM(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"00:00", 0, true},
		{"07:30", 450, true},
		{"23:59", 23*60 + 59, true},
		{" 23:00 ", 23 * 60, true},
		{"24:00", 0, false},
		{"7:30", 450, true}, // 小时允许 1~2 位
		{"07:5", 0, false},  // 分钟必须两位
		{"0730", 0, false},
		{"abc", 0, false},
		{"", 0, false},
		{"07:60", 0, false},
	}
	for _, c := range cases {
		got, ok := parseHHMM(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseHHMM(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func quietAt(hour, minute int) time.Time {
	return time.Date(2026, 10, 14, hour, minute, 0, 0, quietWindowZone)
}

func TestInQuietWindows(t *testing.T) {
	windows := []QuietWindow{
		{Start: "23:00", End: "07:00"}, // 跨天
		{Start: "12:00", End: "13:00"},
	}
	cases := []struct {
		hour, minute int
		want         bool
	}{
		{23, 0, true},
		{23, 59, true},
		{0, 0, true},
		{6, 59, true},
		{7, 0, false},  // end 为开区间
		{11, 59, false},
		{12, 0, true},
		{12, 59, true},
		{13, 0, false},
		{22, 59, false},
	}
	for _, c := range cases {
		if got := inQuietWindows(windows, quietAt(c.hour, c.minute)); got != c.want {
			t.Errorf("inQuietWindows(%02d:%02d) = %v, want %v", c.hour, c.minute, got, c.want)
		}
	}
	if inQuietWindows(nil, quietAt(23, 30)) {
		t.Error("empty windows must never match")
	}
	// start == end 视为全天屏蔽
	if !inQuietWindows([]QuietWindow{{Start: "08:00", End: "08:00"}}, quietAt(8, 0)) {
		t.Error("start == end should block all day")
	}
	// 非法时间忽略不计
	if inQuietWindows([]QuietWindow{{Start: "abc", End: "07:00"}}, quietAt(23, 30)) {
		t.Error("invalid window must be ignored")
	}
	// 时区换算：15:30Z = 北京时间 23:30，落在 23:00-07:00 内
	utcTime := time.Date(2026, 10, 14, 15, 30, 0, 0, time.UTC)
	if !inQuietWindows([]QuietWindow{{Start: "23:00", End: "07:00"}}, utcTime) {
		t.Error("15:30 UTC should match 23:00-07:00 Beijing window")
	}
}

func TestNormalizeQuietWindows(t *testing.T) {
	normalized, msg := normalizeQuietWindows([]QuietWindow{
		{},                        // 双空行丢弃
		{Start: "23:00", End: "07:00"},
	})
	if msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	if len(normalized) != 1 {
		t.Fatalf("expected 1 window, got %d", len(normalized))
	}
	if _, msg := normalizeQuietWindows([]QuietWindow{{Start: "25:00", End: "07:00"}}); msg == "" {
		t.Error("invalid hour must be rejected")
	}
	if _, msg := normalizeQuietWindows([]QuietWindow{{Start: "08:00", End: ""}}); msg == "" {
		t.Error("half-filled window must be rejected")
	}
	if got, _ := normalizeQuietWindows(nil); got == nil {
		t.Error("nil input should normalize to empty slice")
	}
}

func TestStopTestRuns(t *testing.T) {
	r := &TestRunner{
		runningGroups: map[int64]bool{},
		runCancels:    map[int64]context.CancelFunc{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !r.tryLockGroup(1, cancel) {
		t.Fatal("expected group lock to succeed")
	}
	if n := r.StopTestRuns(); n != 1 {
		t.Fatalf("StopTestRuns() = %d, want 1", n)
	}
	select {
	case <-ctx.Done():
	default:
		t.Error("cancel was not invoked")
	}
	r.releaseGroup(1)
	if n := r.StopTestRuns(); n != 0 {
		t.Fatalf("StopTestRuns() after release = %d, want 0", n)
	}
}
