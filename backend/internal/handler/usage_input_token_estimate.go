package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func captureUsageInputTokenEstimate(req *http.Request, body []byte) {
	if req == nil {
		return
	}
	model := ""
	path := ""
	if req.URL != nil {
		path = req.URL.Path
		if _, suffix, ok := strings.Cut(req.URL.Path, "/models/"); ok {
			model, _, _ = strings.Cut(suffix, ":")
		}
	}
	if estimate := service.EstimateUsageInputTokens(body, model, path); estimate != nil {
		*req = *req.WithContext(service.WithUsageInputTokenEstimate(req.Context(), estimate))
	}
}
