package iqdetectd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Probe is a built-in IQ probe question with its accepted answers. All probes
// instruct the model to reply with the final answer only so that grading stays
// deterministic.
type Probe struct {
	ID       string
	Category string
	Prompt   string
	Answers  []string
	Choice   bool // answer is a single A-E letter
}

// instruction is appended to every probe prompt.
const instruction = "\n\nAnswer with the final answer only, no explanation, no units."

// builtinProbes is the fixed probe set of this iqdetectd build.
func builtinProbes() []Probe {
	return []Probe{
		{
			ID:       "arith-001",
			Category: "arithmetic",
			Prompt:   "A shop sells pens at 3 yuan each and notebooks at 7 yuan each. Lin buys 4 pens and 3 notebooks and pays with a 35 yuan note. How much change does he get?",
			Answers:  []string{"2", "2 yuan", "2yuan"},
		},
		{
			ID:       "arith-002",
			Category: "arithmetic",
			Prompt:   "Compute 17 * 23 - 48 / 6.",
			Answers:  []string{"381"},
		},
		{
			ID:       "logic-001",
			Category: "logic",
			Prompt:   "All bloops are razzies. All razzies are lazzies. Are all bloops definitely lazzies? Reply with a single letter: A for yes, B for no.",
			Answers:  []string{"a"},
			Choice:   true,
		},
		{
			ID:       "logic-002",
			Category: "logic",
			Prompt:   "Alice is taller than Bob. Carol is shorter than Bob. Who is the tallest? Reply with a single letter: A for Alice, B for Bob, C for Carol.",
			Answers:  []string{"a"},
			Choice:   true,
		},
		{
			ID:       "pattern-001",
			Category: "pattern",
			Prompt:   "What number comes next in the sequence 2, 6, 12, 20, 30, ...?",
			Answers:  []string{"42"},
		},
		{
			ID:       "pattern-002",
			Category: "pattern",
			Prompt:   "What number comes next in the sequence 1, 1, 2, 3, 5, 8, ...?",
			Answers:  []string{"13"},
		},
		{
			ID:       "count-001",
			Category: "counting",
			Prompt:   "How many times does the letter \"r\" appear in the word \"strawberry\"?",
			Answers:  []string{"3"},
		},
		{
			ID:       "reason-001",
			Category: "reasoning",
			Prompt:   "A farmer has 17 sheep. All but 9 run away. How many sheep does the farmer have left?",
			Answers:  []string{"9"},
		},
		{
			ID:       "reason-002",
			Category: "reasoning",
			Prompt:   "It takes 5 machines 5 minutes to make 5 widgets. How many minutes does it take 100 machines to make 100 widgets? Reply with a number only.",
			Answers:  []string{"5"},
		},
		{
			ID:       "convert-001",
			Category: "conversion",
			Prompt:   "A train travels 180 kilometers in 1.5 hours. What is its average speed in km/h? Reply with a number only.",
			Answers:  []string{"120"},
		},
	}
}

// ProbeInfos exposes the probe set without leaking accepted answers.
func ProbeInfos() []ProbeInfo {
	probes := builtinProbes()
	infos := make([]ProbeInfo, 0, len(probes))
	for _, p := range probes {
		infos = append(infos, ProbeInfo{ID: p.ID, Category: p.Category, Prompt: p.Prompt})
	}
	return infos
}

var numberPattern = regexp.MustCompile(`-?\d+(?:\.\d+)?`)

// gradeAnswer reports whether the model answer is accepted for the probe.
// Grading tries, in order: exact match after normalization, choice-letter
// extraction, then numeric extraction (the last number in the answer).
func gradeAnswer(p Probe, answer string) bool {
	answer = normalizeAnswer(answer)
	if answer == "" {
		return false
	}
	for _, want := range p.Answers {
		if normalizeAnswer(want) == answer {
			return true
		}
	}
	if p.Choice {
		return extractChoiceLetter(answer) != "" && containsFold(p.Answers, extractChoiceLetter(answer))
	}
	// Numeric probes: accept when the last number mentioned equals the target.
	if got, ok := lastNumber(answer); ok {
		for _, want := range p.Answers {
			if n, err := strconv.ParseFloat(normalizeAnswer(want), 64); err == nil && n == got {
				return true
			}
		}
	}
	return false
}

// normalizeAnswer keeps digits and latin letters only, lowercased, so that
// filler such as "答案是" or "The answer is" disappears before comparison.
func normalizeAnswer(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.Is(unicode.Han, r):
			// Drop CJK filler words such as 答案/是; real numeric answers are digits.
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

var choicePattern = regexp.MustCompile(`(?i)\b([abcde])\b|^([abcde])`)

// extractChoiceLetter pulls a standalone A-E letter out of a graded answer.
func extractChoiceLetter(s string) string {
	m := choicePattern.FindStringSubmatch(s)
	if len(m) >= 2 {
		if m[1] != "" {
			return strings.ToLower(m[1])
		}
		if len(m) >= 3 && m[2] != "" {
			return strings.ToLower(m[2])
		}
	}
	return ""
}

// lastNumber returns the last number appearing in the answer text.
func lastNumber(s string) (float64, bool) {
	matches := numberPattern.FindAllString(s, -1)
	if len(matches) == 0 {
		return 0, false
	}
	n, err := strconv.ParseFloat(matches[len(matches)-1], 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func containsFold(list []string, v string) bool {
	for _, item := range list {
		if strings.EqualFold(item, v) {
			return true
		}
	}
	return false
}

// IQScore maps a correct/total ratio onto a 40-140 scale.
func IQScore(correct, total int) float64 {
	if total <= 0 {
		return 0
	}
	iq := 40 + 100*float64(correct)/float64(total)
	return roundHalf(iq)
}

func roundHalf(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// FormatIQ renders a score for human-readable contexts.
func FormatIQ(iq float64) string {
	return fmt.Sprintf("%.1f", iq)
}
