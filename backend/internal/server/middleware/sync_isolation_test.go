package middleware

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSyncIsolationEndpointClassification(t *testing.T) {
	for _, tc := range []struct {
		method, path  string
		stream, count bool
	}{
		{"POST", "/v1/messages", false, true},
		{"POST", "/v1/chat/completions", false, true},
		{"POST", "/backend-api/codex/responses", false, true},
		{"GET", "/v1/responses", true, true},
		{"GET", "/v1/realtime", true, true},
		{"POST", "/v1beta/models/gemini:streamGenerateContent", true, true},
		{"POST", "/v1beta/models/gemini:generateContent", false, true},
		{"POST", "/v1/images/edits", false, true},
		{"GET", "/v1/models", false, false},
		{"POST", "/v1/messages/count_tokens", false, false},
		{"POST", "/v1/responses/input_tokens", false, false},
		{"POST", "/v1/images/generations/async", false, false},
		{"POST", "/v1/images/batches", false, false},
		{"GET", "/v1/images/tasks/123", false, false},
		{"POST", "/v1beta/models/gemini:countTokens", false, false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			stream, count := syncIsolationEndpoint(tc.method, tc.path)
			require.Equal(t, tc.stream, stream)
			require.Equal(t, tc.count, count)
		})
	}
}

func TestSyncIsolationStreamClassification(t *testing.T) {
	for _, tc := range []struct {
		body                    string
		fallback, stream, valid bool
	}{
		{`{"model":"x","stream":true}`, false, true, true},
		{`{"model":"x","stream":false}`, false, false, true},
		{`{"model":"x"}`, false, false, true},
		{`{"contents":[]}`, true, true, true},
		{`{"stream":"true"}`, false, false, false},
		{`{"stream":`, false, false, false},
	} {
		stream, valid := syncIsolationStream([]byte(tc.body), "application/json", tc.fallback)
		require.Equal(t, tc.stream, stream, tc.body)
		require.Equal(t, tc.valid, valid, tc.body)
	}
	body := "--test\r\nContent-Disposition: form-data; name=\"stream\"\r\n\r\ntrue\r\n--test--\r\n"
	stream, valid := syncIsolationStream([]byte(body), "multipart/form-data; boundary=test", false)
	require.True(t, valid)
	require.True(t, stream)
}

func TestSyncIsolationEffectiveStreamUsesEndpoint(t *testing.T) {
	require.True(t, syncIsolationEffectiveStream("/v1beta/models/gemini:streamGenerateContent", false))
	for _, path := range []string{"/v1beta/models/gemini:generateContent", "/v1/systemone", "/v1/embeddings", "/v1/stt", "/v1/alpha/search", "/v1/web_search", "/v1/x_search"} {
		require.False(t, syncIsolationEffectiveStream(path, true), path)
	}
	require.True(t, syncIsolationEffectiveStream("/v1/responses", true))
	require.False(t, syncIsolationEffectiveStream("/v1/messages", false))
}

type syncIsolationObservationRepo struct {
	service.GroupRepository
	streams chan bool
}

func (r *syncIsolationObservationRepo) ObserveSyncIsolation(_ context.Context, _, _, groupID int64, stream bool, _ time.Time) (int64, bool, error) {
	r.streams <- stream
	return groupID, false, nil
}

func TestSyncIsolationRequestPreservesBodyAndUsesProtocol(t *testing.T) {
	repo := &syncIsolationObservationRepo{streams: make(chan bool, 4)}
	svc := service.NewAPIKeyService(nil, nil, repo, nil, nil, nil, nil)
	t.Cleanup(svc.StopSyncIsolation)
	groupID := int64(3)
	key := &service.APIKey{ID: 1, UserID: 2, GroupID: &groupID, User: &service.User{ID: 2}, Group: &service.Group{ID: groupID, Status: service.StatusActive, SyncIsolationEnabled: true}}
	for _, tc := range []struct {
		path, body string
		stream     bool
	}{
		{"/v1/messages", "\xef\xbb\xbf{\"model\":\"x\",\"text\":\"line\nline\",\"stream\":true}", true},
		{"/v1beta/models/gemini:streamGenerateContent", `{"contents":[],"stream":false}`, true},
		{"/v1beta/models/gemini:generateContent", `{"contents":[],"stream":true}`, false},
		{"/v1/embeddings", `{"model":"x","stream":true}`, false},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		c.Request.Header.Set("Content-Type", "application/json")
		observeSyncIsolationRequest(c, svc, key, nil)
		select {
		case stream := <-repo.streams:
			require.Equal(t, tc.stream, stream, tc.path)
		case <-time.After(time.Second):
			t.Fatal("background observation did not run")
		}
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.True(t, gjson.ValidBytes(body), "downstream must receive reusable normalized JSON")
	}
}
