package lottery

import (
	"strings"
	"time"
)

const (
	StageBeforeStart = "before_start"
	StageStarted     = "started"
	StageBeforeDraw  = "before_draw"
	StageResults     = "results"
)

// AllNotifyStages 按发送顺序排列的全部阶段。
func AllNotifyStages() []string {
	return []string{StageBeforeStart, StageStarted, StageBeforeDraw, StageResults}
}

// NormalizeNotifyStages 丢掉未知阶段，并按固定顺序去重。
func NormalizeNotifyStages(stages []string) []string {
	want := map[string]struct{}{}
	for _, stage := range stages {
		switch strings.TrimSpace(stage) {
		case StageBeforeStart, StageStarted, StageBeforeDraw, StageResults:
			want[strings.TrimSpace(stage)] = struct{}{}
		}
	}
	out := make([]string, 0, len(want))
	for _, stage := range AllNotifyStages() {
		if _, ok := want[stage]; ok {
			out = append(out, stage)
		}
	}
	return out
}

// ResolveNotifyStages 把请求里的阶段列表变成要保存的列表。
// stages 为 nil 且 legacyAll 为 true 时，沿用旧的“打开即四个阶段都发”。
// 显式传入的列表（包括空列表）只保留勾选的阶段。
func ResolveNotifyStages(stages []string, legacyAll bool) []string {
	if stages != nil {
		return NormalizeNotifyStages(stages)
	}
	if legacyAll {
		return AllNotifyStages()
	}
	return []string{}
}

// DueNotifyStages 返回这场活动此刻该发、且还没发过的通知阶段。
// 已关闭且尚未开奖的场次不再补发。开奖结果只在开奖之后出现。
func DueNotifyStages(a *Activity, now time.Time, sent map[string]struct{}) []string {
	if a == nil || (a.Status == "archived" && a.DrawnAt.IsZero()) {
		return nil
	}
	var out []string
	if _, ok := sent[StageBeforeStart]; !ok && now.Before(a.StartsAt) && !now.Before(a.StartsAt.Add(-10*time.Minute)) {
		out = append(out, StageBeforeStart)
	}
	if _, ok := sent[StageStarted]; !ok && a.DrawnAt.IsZero() && !now.Before(a.StartsAt) && now.Before(a.DrawsAt) {
		out = append(out, StageStarted)
	}
	if _, ok := sent[StageBeforeDraw]; !ok && a.DrawnAt.IsZero() && !now.Before(a.DrawsAt.Add(-5*time.Minute)) && now.Before(a.DrawsAt) {
		out = append(out, StageBeforeDraw)
	}
	if _, ok := sent[StageResults]; !ok && !a.DrawnAt.IsZero() {
		out = append(out, StageResults)
	}
	return out
}
