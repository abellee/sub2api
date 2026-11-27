package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"lotteryd/api/events"
)

const gameplayLoginUserKey = "gameplay_login_user_id"

// Call only after all login checks and token creation have succeeded.
func recordGameplayLogin(c *gin.Context, userID int64) {
	defer func() { _ = recover() }()
	if !events.Enabled() || userID <= 0 || c.GetBool("gameplay_login_emitted") {
		return
	}
	c.Set("gameplay_login_emitted", true)
	ref, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	events.Publish("user.logged_in", userID, ref, map[string]string{"endpoint": c.FullPath(), "platform": c.GetString("auth_method")})
}

func recordGameplayLoginResponse(c *gin.Context) {
	if value, ok := c.Get(gameplayLoginUserKey); ok {
		if id, ok := value.(int64); ok {
			recordGameplayLogin(c, id)
		}
	}
}
