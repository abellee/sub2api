package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func thinkingCompatContext(cfg GroupAnthropicThinkingCompat) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("api_key", &APIKey{Group: &Group{Platform: PlatformAnthropic, AnthropicThinkingCompat: cfg}})
	return c
}

func TestGroupAnthropicThinkingCompat(t *testing.T) {
	all := GroupAnthropicThinkingCompat{Enabled: true, AllModels: true}
	selected := GroupAnthropicThinkingCompat{Enabled: true, Models: []string{"claude-sonnet-5-5"}}
	for _, tt := range []struct {
		name, requested, upstream, thinking, want string
		cfg                                       GroupAnthropicThinkingCompat
	}{
		{"off", "claude-sonnet-5-5", "claude-sonnet-5-5", "enabled", "enabled", GroupAnthropicThinkingCompat{}},
		{"enabled", "claude-sonnet-5-5", "claude-sonnet-5-5", "enabled", "adaptive", all},
		{"disabled", "claude-sonnet-5-5", "claude-sonnet-5-5", "disabled", "between_tools", all},
		{"selected thinking suffix", "claude-sonnet-5-5-thinking", "claude-sonnet-5-5", "enabled", "adaptive", selected},
		{"not selected", "claude-opus-5-5", "claude-opus-5-5", "enabled", "enabled", selected},
		{"wildcard", "friendly-sonnet", "claude-sonnet-5-5", "enabled", "adaptive", GroupAnthropicThinkingCompat{Enabled: true, Models: []string{"friendly-*"}}},
		{"scope uses requested", "friendly-sonnet", "claude-sonnet-5-5", "enabled", "enabled", selected},
		{"old model", "claude-sonnet-4-6", "claude-sonnet-4-6", "enabled", "enabled", all},
		{"mapped old model", "claude-sonnet-5-5", "claude-sonnet-4-6", "enabled", "enabled", all},
		{"opus47", "claude-opus-4-7", "claude-opus-4-7", "enabled", "adaptive", all},
		{"opus55", "claude-opus-5-5", "claude-opus-5-5", "enabled", "adaptive", all},
		{"haiku55", "claude-haiku-5-5", "claude-haiku-5-5", "enabled", "adaptive", all},
		{"fable52", "claude-fable-5-2", "claude-fable-5-2", "enabled", "adaptive", all},
		{"mythos52", "claude-mythos-5-2", "claude-mythos-5-2", "enabled", "adaptive", all},
		{"haiku disabled stays", "claude-haiku-5-5", "claude-haiku-5-5", "disabled", "disabled", all},
		{"adaptive stays", "claude-sonnet-5-5", "claude-sonnet-5-5", "adaptive", "adaptive", all},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"thinking":{"type":%q,"budget_tokens":1000},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"history","signature":"signed"}]}]}`, tt.requested, tt.thinking))
			original := append([]byte(nil), body...)
			got := applyGroupAnthropicThinkingCompat(thinkingCompatContext(tt.cfg), body, tt.requested, tt.upstream)
			require.Equal(t, tt.want, gjson.GetBytes(got, "thinking.type").String())
			require.Equal(t, original, body, "must not mutate a body reused by another account attempt")
			require.Equal(t, gjson.GetBytes(body, "messages").Raw, gjson.GetBytes(got, "messages").Raw)
			if tt.want != tt.thinking {
				require.False(t, gjson.GetBytes(got, "thinking.budget_tokens").Exists())
			} else {
				require.Equal(t, body, got)
			}
		})
	}
	missingKey, _ := gin.CreateTestContext(httptest.NewRecorder())
	wrongPlatform := thinkingCompatContext(all)
	wrongPlatform.MustGet("api_key").(*APIKey).Group.Platform = PlatformOpenAI
	for _, c := range []*gin.Context{nil, missingKey, wrongPlatform} {
		body := []byte(`{"thinking":{"type":"enabled"}}`)
		require.Equal(t, body, applyGroupAnthropicThinkingCompat(c, body, "claude-sonnet-5-5", "claude-sonnet-5-5"))
	}
}

func TestGroupAnthropicThinkingCompatRetainsValidation(t *testing.T) {
	c := thinkingCompatContext(GroupAnthropicThinkingCompat{Enabled: true, AllModels: true})
	for _, extra := range []string{`"output_config":{"effort":"xhigh"}`, `"output_config":{"effort":"max"}`, `"temperature":0.5`, `"tool_choice":{"type":"any"}`} {
		body := []byte(`{"thinking":{"type":"disabled"},` + extra + `}`)
		converted := applyGroupAnthropicThinkingCompat(c, body, "claude-sonnet-5-5", "claude-sonnet-5-5")
		require.Error(t, validateClaude55Request(converted, "claude-sonnet-5-5"))
	}
	for _, body := range []string{`{"messages":[]}`, `{"thinking":null}`, `{"thinking":"enabled"}`} {
		require.Equal(t, body, string(applyGroupAnthropicThinkingCompat(c, []byte(body), "claude-sonnet-5-5", "claude-sonnet-5-5")))
	}
}

// Exercise the actual forwarding boundaries, including model mapping and count_tokens.
func TestGatewayService_GroupThinkingCompatForward(t *testing.T) {
	for _, countTokens := range []bool{false, true} {
		for _, thinking := range []string{"enabled", "disabled"} {
			t.Run(fmt.Sprintf("count=%t/%s", countTokens, thinking), func(t *testing.T) {
				c := thinkingCompatContext(GroupAnthropicThinkingCompat{Enabled: true, Models: []string{"friendly-*"}})
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				body := []byte(fmt.Sprintf(`{"model":"friendly-sonnet","max_tokens":4096,"thinking":{"type":%q,"budget_tokens":1000},"messages":[{"role":"user","content":"hello"}]}`, thinking))
				parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
				require.NoError(t, err)
				account := newAnthropicAPIKeyAccountForTest()
				account.Credentials["model_mapping"] = map[string]any{"friendly-sonnet": "claude-sonnet-5-5"}
				response := `{"id":"msg-1","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":1}}`
				if countTokens {
					response = `{"input_tokens":5}`
				}
				upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(response))}}
				cfg := &config.Config{}
				svc := &GatewayService{cfg: cfg, httpUpstream: upstream, responseHeaderFilter: compileResponseHeaderFilter(cfg), rateLimitService: &RateLimitService{}, deferredService: &DeferredService{}}
				if countTokens {
					require.NoError(t, svc.ForwardCountTokens(context.Background(), c, account, parsed))
				} else {
					_, err = svc.Forward(context.Background(), c, account, parsed)
					require.NoError(t, err)
				}
				want := "adaptive"
				if thinking == "disabled" {
					want = "between_tools"
				}
				require.Equal(t, want, gjson.GetBytes(upstream.lastBody, "thinking.type").String())
				require.False(t, gjson.GetBytes(upstream.lastBody, "thinking.budget_tokens").Exists())
				require.True(t, parsed.ThinkingEnabled, "derived parsed fields must be refreshed")
				require.Equal(t, "claude-sonnet-5-5", gjson.GetBytes(upstream.lastBody, "model").String())
			})
		}
	}
}

func TestAPIKeyAuthSnapshotGroupThinkingCompatRoundtrip(t *testing.T) {
	id := int64(9)
	cfg := GroupAnthropicThinkingCompat{Enabled: true, Models: []string{"claude-sonnet-*"}}
	key := &APIKey{ID: 1, UserID: 2, GroupID: &id, Key: "sk-thinking", Status: StatusActive, User: &User{ID: 2, Status: StatusActive}, Group: &Group{ID: id, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true, AnthropicThinkingCompat: cfg}}
	svc := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))
	got, used, err := svc.applyAuthCacheEntry(key.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.Equal(t, cfg, got.Group.AnthropicThinkingCompat)
}

func TestGatewayService_GroupThinkingCompatVertexMapping(t *testing.T) {
	c := thinkingCompatContext(GroupAnthropicThinkingCompat{Enabled: true, Models: []string{"friendly-sonnet"}})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(`{"model":"friendly-sonnet","thinking":{"type":"enabled","budget_tokens":1000},"temperature":0.5,"messages":[{"role":"user","content":"hello"}]}`)), PlatformAnthropic)
	require.NoError(t, err)
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeServiceAccount, Credentials: map[string]any{"model_mapping": map[string]any{"friendly-sonnet": "claude-sonnet-5-5"}}}
	_, err = (&GatewayService{}).Forward(context.Background(), c, account, parsed)
	require.ErrorContains(t, err, "non-default temperature", "validation must still reject incompatible settings before any upstream call")
	require.Equal(t, "adaptive", gjson.GetBytes(parsed.Body.Bytes(), "thinking.type").String())
	require.False(t, gjson.GetBytes(parsed.Body.Bytes(), "thinking.budget_tokens").Exists())
}
