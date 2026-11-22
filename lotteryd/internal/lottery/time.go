package lottery

import (
	"sync"
	"time"
)

// 业务时间一律用北京时间。中国自 1991 年起不实行夏令时，加载失败时退回固定 UTC+8。
var (
	beijingOnce sync.Once
	beijingLoc  *time.Location
)

// Beijing 返回 Asia/Shanghai。
func Beijing() *time.Location {
	beijingOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			loc = time.FixedZone("Asia/Shanghai", 8*3600)
		}
		beijingLoc = loc
	})
	return beijingLoc
}

// InBeijing 把时刻换算到北京时间，供日历日和钟点使用。
func InBeijing(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(Beijing())
}
