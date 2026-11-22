package handler

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAICyberPolicyRequestPayloadHash(t *testing.T) {
	body := []byte("request payload")
	require.Empty(t, openAICyberPolicyRequestPayloadHash(nil, body))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.Empty(t, openAICyberPolicyRequestPayloadHash(c, body))
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{UpstreamStatus: 400})
	require.Equal(t, service.HashUsageRequestPayload(body), openAICyberPolicyRequestPayloadHash(c, body))
	require.False(t, c.GetBool(cyberPolicyRecordedKey), "computing the hash must not mark the event recorded")
	c.Set(cyberPolicyRecordedKey, true)
	require.Empty(t, openAICyberPolicyRequestPayloadHash(c, body))
}

var openAICPUHashBenchmarkSink string

func BenchmarkOpenAICPUCyberHash(b *testing.B) {
	body := bytes.Repeat([]byte("A"), 8<<20)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	b.Run("Previous", func(b *testing.B) {
		b.SetBytes(int64(len(body)))
		b.ReportAllocs()
		for b.Loop() {
			openAICPUHashBenchmarkSink = service.HashUsageRequestPayload(body)
		}
	})
	b.Run("Unmarked", func(b *testing.B) {
		b.SetBytes(int64(len(body)))
		b.ReportAllocs()
		for b.Loop() {
			openAICPUHashBenchmarkSink = openAICyberPolicyRequestPayloadHash(c, body)
		}
	})
	b.Run("Marked", func(b *testing.B) {
		service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{UpstreamStatus: 400})
		b.SetBytes(int64(len(body)))
		b.ReportAllocs()
		for b.Loop() {
			openAICPUHashBenchmarkSink = openAICyberPolicyRequestPayloadHash(c, body)
		}
	})
}
