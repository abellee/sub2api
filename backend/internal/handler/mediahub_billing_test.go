package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
)

type mediaResultSinkFunc func(mediahub.SettleRequest) error

func (f mediaResultSinkFunc) Settle(req mediahub.SettleRequest) error { return f(req) }

func TestMediaHubVideoUsesMainSiteVideoFields(t *testing.T) {
	req := mediahub.SettleRequest{RequestID: "media_task_settle:mtask_1", Model: "grok-imagine-video-1.5", Usage: &mediahub.Usage{Unit: mediahub.UnitSecond, Quantity: 10}, Parameters: map[string]any{"resolution": "720p"}}
	r, err := mediaHubUsageResult(req)
	if err != nil {
		t.Fatal(err)
	}
	if r.VideoCount != 1 || r.VideoDurationSeconds != 10 || r.VideoResolution != "720p" || r.ImageCount != 0 || r.RequestID != req.RequestID {
		t.Fatalf("video billing mapping=%+v", r)
	}
	req.Usage.Quantity = 20
	if _, err := mediaHubUsageResult(req); err == nil {
		t.Fatal("must not silently clamp long videos to Grok's 15 second billing range")
	}
	req.Usage = &mediahub.Usage{Unit: mediahub.UnitImage, Quantity: 3}
	r, err = mediaHubUsageResult(req)
	if err != nil || r.ImageCount != 3 || !r.ImageEndpoint || r.VideoCount != 0 {
		t.Fatalf("image billing mapping=%+v %v", r, err)
	}
}
