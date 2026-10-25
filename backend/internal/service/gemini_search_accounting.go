package service

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const geminiSearchCounterKey = "gemini_google_search_counter"

// Google grounding metadata is cumulative and may be repeated in SSE events or
// candidates. Keep the greatest multiplicity of each query, never add snapshots.
// Queries seen in distinct events are retained even if the final event omits them.
type geminiSearchCounter map[string]int

func beginGeminiSearchObservation(c *gin.Context) {
	if c != nil {
		c.Set(geminiSearchCounterKey, geminiSearchCounter{})
	}
}

func observeGeminiSearch(c *gin.Context, payload []byte) {
	if c == nil {
		return
	}
	v, ok := c.Get(geminiSearchCounterKey)
	if !ok {
		return
	}
	counter := v.(geminiSearchCounter)
	root := gjson.ParseBytes(payload)
	if response := root.Get("response"); response.Exists() {
		root = response
	}
	for _, candidate := range root.Get("candidates").Array() {
		queries := candidate.Get("groundingMetadata.webSearchQueries")
		if !queries.Exists() {
			queries = candidate.Get("grounding_metadata.web_search_queries")
		}
		counts := map[string]int{}
		for _, query := range queries.Array() {
			if query.Type == gjson.String {
				if q := strings.TrimSpace(query.String()); q != "" {
					counts[q]++
				}
			}
		}
		for query, count := range counts {
			if count > counter[query] {
				counter[query] = count
			}
		}
	}
}

func observedGeminiSearches(c *gin.Context) int {
	if c == nil {
		return 0
	}
	v, ok := c.Get(geminiSearchCounterKey)
	if !ok {
		return 0
	}
	total := 0
	for _, count := range v.(geminiSearchCounter) {
		total += count
	}
	return total
}

func finishGeminiSearchObservation(c *gin.Context, result *ForwardResult) {
	if result != nil {
		result.SearchCount = observedGeminiSearches(c)
		result.SearchProvider = "google"
	}
}
