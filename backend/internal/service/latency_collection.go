package service

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/latencytrace"
	"github.com/gin-gonic/gin"
)

func markLatencyStreamEvent(c *gin.Context, semantic bool) {
	if c == nil || c.Request == nil {
		return
	}
	trace := latencytrace.From(c.Request.Context())
	if trace == nil {
		return
	}
	trace.MarkCurrent("first_stream_event")
	if semantic {
		trace.MarkCurrent("first_semantic_output")
	}
}
