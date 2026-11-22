package service

import (
	"bytes"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func normalizeGroupAnthropicThinkingCompat(platform string, cfg GroupAnthropicThinkingCompat) (GroupAnthropicThinkingCompat, error) {
	if platform != PlatformAnthropic {
		return GroupAnthropicThinkingCompat{}, nil
	}
	models, err := normalizeGroupModelAllowlist(GroupModelAllowlist{Models: cfg.Models})
	if err != nil {
		return GroupAnthropicThinkingCompat{}, err
	}
	cfg.Models = models.Models
	if cfg.Enabled && !cfg.AllModels && len(cfg.Models) == 0 {
		return cfg, infraerrors.New(http.StatusBadRequest, "INVALID_ANTHROPIC_THINKING_COMPAT", "select at least one model for thinking conversion")
	}
	return cfg, nil
}

// Match the requested model, then convert according to the actual upstream model.
func applyGroupAnthropicThinkingCompat(c *gin.Context, body []byte, requestedModel, upstreamModel string) []byte {
	if c == nil {
		return body
	}
	value, _ := c.Get("api_key")
	key, _ := value.(*APIKey)
	if key == nil || key.Group == nil || key.Group.Platform != PlatformAnthropic {
		return body
	}
	cfg := key.Group.AnthropicThinkingCompat
	if !cfg.Enabled {
		return body
	}
	if !cfg.AllModels && !(GroupModelAllowlist{Enabled: true, Models: cfg.Models}).Allows(requestedModel) {
		return body
	}
	// Older models keep their existing thinking format; enabling all models must not break them.
	if !isClaude55SignedThinkingModel(upstreamModel) && !isBedrockOpus47OrNewer(upstreamModel) {
		return body
	}
	thinking := gjson.GetBytes(body, "thinking")
	if !thinking.IsObject() {
		return body
	}
	switch thinking.Get("type").String() {
	case "enabled":
		body, _ = sjson.SetBytes(body, "thinking.type", "adaptive")
		body, _ = sjson.DeleteBytes(body, "thinking.budget_tokens")
	case "disabled":
		if claude.IsSonnet55(upstreamModel) {
			body, _ = sjson.SetBytes(body, "thinking.type", "between_tools")
			body, _ = sjson.DeleteBytes(body, "thinking.budget_tokens")
		}
	}
	return body
}

func applyParsedGroupAnthropicThinkingCompat(c *gin.Context, parsed *ParsedRequest, upstreamModel string) error {
	body := parsed.Body.Bytes()
	converted := applyGroupAnthropicThinkingCompat(c, body, parsed.Model, upstreamModel)
	if bytes.Equal(body, converted) {
		return nil
	}
	return parsed.ReplaceBody(converted)
}
