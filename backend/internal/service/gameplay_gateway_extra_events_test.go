package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"lotteryd/api/events"
)

func TestGameplayFirstOutputIgnoresHeartbeatsAndSnapshotsOnce(t *testing.T) {
	var captured []events.Event
	events.SetSink(func(e events.Event) { captured = append(captured, e) })
	t.Cleanup(func() { events.SetSink(nil) })
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	c.Set("api_key", &APIKey{ID: 7, UserID: 2})
	c.Set("ops_model", "test-model")
	observeGameplayAnthropicOutput(c, `{"type":"ping"}`)
	observeGameplayChatOutput(c, `{"choices":[{"delta":{"role":"assistant"}}]}`)
	observeGameplayGeminiOutput(c, []byte(`{"usageMetadata":{"promptTokenCount":1}}`))
	if len(captured) != 0 {
		t.Fatal("metadata started first-output event")
	}
	observeGameplayAnthropicOutput(c, `{"type":"content_block_delta","delta":{"text":"hello"}}`)
	observeGameplayChatOutput(c, `{"choices":[{"delta":{"content":"again"}}]}`)
	if len(captured) != 1 || captured[0].Type != "request.first_output" || captured[0].UserID != 2 {
		t.Fatalf("events=%+v", captured)
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)
	c.Set("gameplay_first_output", false)
	observeGameplayFirstOutput(c)
	if len(captured) != 1 {
		t.Fatal("canceled request emitted output")
	}
}
