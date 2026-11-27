package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"lotteryd/api/events"
)

func emitGatewayEvent(c *gin.Context, kind string, accountID int64) {
	if !events.Enabled() || c == nil || c.Request == nil {
		return
	}
	key := getOpsAPIKey(c)
	if key == nil {
		return
	}
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	data := map[string]string{"api_key_id": events.S(key.ID), "account_id": events.S(accountID), "model": c.GetString(opsModelKey), "endpoint": c.FullPath()}
	if key.GroupID != nil {
		data["group_id"] = events.S(*key.GroupID)
	}
	events.Publish(kind, key.UserID, requestID+":"+events.S(c.GetInt("gameplay_attempt")), data)
}
