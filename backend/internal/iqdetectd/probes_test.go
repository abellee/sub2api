package iqdetectd

import "testing"

func TestGradeAnswerExactAndFillers(t *testing.T) {
	probes := builtinProbes()
	byID := make(map[string]Probe, len(probes))
	for _, p := range probes {
		byID[p.ID] = p
	}

	cases := []struct {
		probeID string
		answer  string
		want    bool
	}{
		{"arith-001", "2", true},
		{"arith-001", "The change is 2 yuan.", true},
		{"arith-001", "答案是 2", true},
		{"arith-001", "3", false},
		{"arith-002", "381", true},
		{"arith-002", "381.", true},
		{"logic-001", "A", true},
		{"logic-001", "a", true},
		{"logic-001", "A) yes", true},
		{"logic-001", "B", false},
		{"pattern-001", "42", true},
		{"pattern-001", "The next number is 42", true},
		{"count-001", "3", true},
		{"count-001", "three", false},
		{"reason-002", "5", true},
		{"reason-002", "It takes 5 minutes.", true},
		{"", "", false},
	}
	for _, tc := range cases {
		if tc.probeID == "" {
			if gradeAnswer(Probe{Answers: []string{"1"}}, tc.answer) {
				t.Fatalf("empty probe should not grade")
			}
			continue
		}
		if got := gradeAnswer(byID[tc.probeID], tc.answer); got != tc.want {
			t.Errorf("gradeAnswer(%s, %q) = %v, want %v", tc.probeID, tc.answer, got, tc.want)
		}
	}
}

func TestIQScore(t *testing.T) {
	cases := []struct {
		correct, total int
		want           float64
	}{
		{0, 10, 40},
		{10, 10, 140},
		{5, 10, 90},
		{7, 10, 110},
		{0, 0, 0},
	}
	for _, tc := range cases {
		if got := IQScore(tc.correct, tc.total); got != tc.want {
			t.Errorf("IQScore(%d,%d) = %v, want %v", tc.correct, tc.total, got, tc.want)
		}
	}
}

func TestChatCompletionsURL(t *testing.T) {
	cases := []struct {
		base, want string
	}{
		{"http://127.0.0.1:8080", "http://127.0.0.1:8080/v1/chat/completions"},
		{"http://127.0.0.1:8080/", "http://127.0.0.1:8080/v1/chat/completions"},
		{"https://api.example.com/v1", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/v1/", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/v1/chat/completions", "https://api.example.com/v1/chat/completions"},
	}
	for _, tc := range cases {
		r := &Runner{UpstreamBase: tc.base}
		if got := r.chatCompletionsURL(); got != tc.want {
			t.Errorf("chatCompletionsURL(%q) = %q, want %q", tc.base, got, tc.want)
		}
	}
}

func TestProbeInfosHidesAnswers(t *testing.T) {
	infos := ProbeInfos()
	if len(infos) == 0 {
		t.Fatal("expected probes")
	}
	for _, info := range infos {
		if info.Prompt == "" || info.ID == "" || info.Category == "" {
			t.Errorf("probe %q incomplete: %+v", info.ID, info)
		}
	}
}
