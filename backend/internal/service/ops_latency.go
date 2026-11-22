package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/latencytrace"

func (s *OpsService) LatencyCollector() *latencytrace.Collector {
	if s == nil {
		return nil
	}
	return s.latencyCollector
}
