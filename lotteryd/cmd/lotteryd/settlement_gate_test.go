package main

import "testing"

func TestSettlementDisabled(t *testing.T) {
	cases := []struct {
		mode, version string
		want          bool
	}{
		{"auto", "dev", true},
		{"", "dev", true},
		{"auto", "", true},
		{"auto", "0.1.3", false},
		{"on", "dev", false},
		{"off", "0.1.3", true},
		{"OFF", "0.1.3", true},
		{"true", "dev", false},
		{"false", "0.1.3", true},
	}
	for _, tc := range cases {
		if got := settlementDisabled(tc.mode, tc.version); got != tc.want {
			t.Errorf("settlementDisabled(%q, %q) = %v, want %v", tc.mode, tc.version, got, tc.want)
		}
	}
}
