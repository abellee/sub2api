package domain

// GroupAnthropicThinkingCompat controls legacy thinking conversion for an Anthropic group.
type GroupAnthropicThinkingCompat struct {
	Enabled   bool     `json:"enabled"`
	AllModels bool     `json:"all_models"`
	Models    []string `json:"models,omitempty"`
}
