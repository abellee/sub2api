package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) SetContentSafetyBilling(s *service.ContentSafetyBillingService) {
	h.contentSafetyBilling = s
}

func (h *SettingHandler) GetContentSafetyBilling(c *gin.Context) {
	if h.contentSafetyBilling == nil {
		response.Error(c, http.StatusServiceUnavailable, "Content safety billing unavailable")
		return
	}
	config, err := h.contentSafetyBilling.Config(c.Request.Context())
	if err != nil {
		response.Error(c, 500, "Failed to load content safety billing")
		return
	}
	response.Success(c, config)
}

func (h *SettingHandler) UpdateContentSafetyBilling(c *gin.Context) {
	if h.contentSafetyBilling == nil {
		response.Error(c, http.StatusServiceUnavailable, "Content safety billing unavailable")
		return
	}
	var req struct {
		Enabled          *bool    `json:"enabled"`
		Amount           *float64 `json:"amount"`
		MatchDescription *string  `json:"match_description"`
		ClientMessage    *string  `json:"client_message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil || req.Amount == nil || req.MatchDescription == nil || req.ClientMessage == nil {
		response.BadRequest(c, "All content safety billing fields are required")
		return
	}
	config := service.ContentSafetyBillingConfig{Enabled: *req.Enabled, Amount: *req.Amount, MatchDescription: *req.MatchDescription, ClientMessage: *req.ClientMessage}
	if err := config.Validate(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.contentSafetyBilling.UpdateConfig(c.Request.Context(), config); err != nil {
		response.Error(c, 500, "Failed to save content safety billing")
		return
	}
	response.Success(c, config)
}

func (h *SettingHandler) ListContentSafetyDebits(c *gin.Context) {
	if h.contentSafetyBilling == nil {
		response.Error(c, http.StatusServiceUnavailable, "Content safety billing unavailable")
		return
	}
	parse := func(key string, fallback, max int) (int, error) {
		value := c.Query(key)
		if value == "" {
			return fallback, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > max {
			return 0, errors.New("invalid pagination")
		}
		return n, nil
	}
	page, err := parse("page", 1, 1000000)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	size, err := parse("page_size", 20, 100)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	records, err := h.contentSafetyBilling.Records(c.Request.Context(), page, size)
	if err != nil {
		response.Error(c, 500, "Failed to load content safety debits")
		return
	}
	response.Success(c, records)
}
