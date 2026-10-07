package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lotteryd/internal/app"
	"lotteryd/internal/lottery"
	"lotteryd/internal/store"
)

// Server is the lotteryd HTTP API.
type Server struct {
	App       *app.App
	JWTSecret string
	StartedAt time.Time
	Version   string
	// AuthMode: "local"（默认，用共享 jwt.secret 本地校验）或
	// "introspect"（每个请求调主服务 /auth/me 确认身份，无需共享 secret）。
	AuthMode string
	wsHub    *wsHub
}

// New creates the HTTP server.
func New(a *app.App, jwtSecret, version string) *Server {
	sv := &Server{App: a, JWTSecret: jwtSecret, StartedAt: time.Now().UTC(), Version: version}
	sv.wsHub = newWSHub(sv)
	return sv
}

// RunWSHub 启动 WebSocket 推送循环（cmd/lotteryd 里放独立 goroutine）。
func (s *Server) RunWSHub(ctx context.Context) {
	s.wsHub.Run(ctx)
}

// SetAuthMode configures the authentication mode ("local" or "introspect").
func (s *Server) SetAuthMode(mode string) {
	if mode == "introspect" {
		s.AuthMode = "introspect"
	}
}

// ---- Response helpers (Sub2API-style envelope) ----

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Code: 0, Data: data})
}

func fail(w http.ResponseWriter, status int, code int, message string) {
	writeJSON(w, status, envelope{Code: code, Message: message})
}

// ---- Middleware ----

func (s *Server) cors(next http.Handler) http.Handler {
	// 同源部署无需配置；跨域组件（demo 壳 / 独立部署页）通过 -cors-origins 放行。
	allowed := corsOrigins
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (len(allowed) == 0 || containsOrigin(allowed, origin)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// corsOrigins is set at startup via SetCORSOrigins.
var corsOrigins []string

// SetCORSOrigins configures the allowed CORS origins (empty = same-origin only).
func SetCORSOrigins(origins []string) { corsOrigins = origins }

func containsOrigin(list []string, origin string) bool {
	for _, o := range list {
		if o == "*" || strings.EqualFold(strings.TrimSpace(o), origin) {
			return true
		}
	}
	return false
}

type authedHandler func(w http.ResponseWriter, r *http.Request, claims *Claims)

func (s *Server) requireUser(next authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := s.bearerClaims(r)
		if err != nil {
			fail(w, http.StatusUnauthorized, 401, err.Error())
			return
		}
		next(w, r, claims)
	})
}

func (s *Server) requireAdmin(next authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := s.bearerClaims(r)
		if err != nil {
			fail(w, http.StatusUnauthorized, 401, err.Error())
			return
		}
		if claims.Role != "admin" {
			fail(w, http.StatusForbidden, 403, "admin role required")
			return
		}
		next(w, r, claims)
	})
}

func (s *Server) bearerClaims(r *http.Request) (*Claims, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, errors.New("missing bearer token")
	}
	return s.claimsFromToken(r.Context(), strings.TrimPrefix(h, "Bearer "))
}

// claimsFromToken 解析 token 身份（HTTP 与 WebSocket 共用）。
func (s *Server) claimsFromToken(ctx context.Context, token string) (*Claims, error) {
	// introspect 模式：不共享 jwt.secret，直接用用户 token 调主服务 /auth/me 确认身份。
	if s.AuthMode == "introspect" {
		id, err := s.App.Sub2API.Introspect(ctx, token)
		if err != nil {
			return nil, errors.New("identity verification failed")
		}
		return &Claims{UserID: id.UserID, Email: id.Email, Role: id.Role, RegisteredAt: id.RegisteredAt}, nil
	}
	return ParseJWT(token, s.JWTSecret)
}

// ---- Routing ----

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)

	// User endpoints.
	mux.Handle("GET /v1/activities", s.requireUser(s.handleListActivities))
	mux.Handle("GET /v1/activities/{id}/winners", s.requireUser(s.handleActivityWinners))
	mux.Handle("POST /v1/activities/{id}/participate", s.requireUser(s.handleParticipate))
	mux.Handle("GET /v1/my/winnings", s.requireUser(s.handleMyWinnings))
	mux.Handle("GET /v1/eligibility", s.requireUser(s.handleEligibility))
	// WS 路由注册两个形态：/v1/ws（独立前端壳直连）与 /lotteryd/v1/ws
	//（并入主站后经 vite/nginx 代理转发，部分代理对 upgrade 请求不应用 rewrite）。
	mux.Handle("GET /v1/ws", http.HandlerFunc(s.ServeWS))
	mux.Handle("GET /lotteryd/v1/ws", http.HandlerFunc(s.ServeWS))
	mux.Handle("GET /v1/me/visibility", s.requireUser(s.handleMyVisibility))

	// 任务模块（用户侧）。
	mux.Handle("GET /v1/tasks", s.requireUser(s.handleListTasks))
	mux.Handle("GET /v1/task-prompts", s.requireUser(s.handleTaskPrompts))
	mux.Handle("GET /v1/me/tasks/phase", s.requireUser(s.handleMyTasksPhase))
	mux.Handle("GET /v1/me/task-visibility", s.requireUser(s.handleMyTaskVisibility))
	mux.Handle("GET /v1/me/task-rewards", s.requireUser(s.handleMyTaskRewards))

	// Admin endpoints.
	mux.Handle("GET /v1/admin/activities", s.requireAdmin(s.handleAdminListActivities))
	mux.Handle("POST /v1/admin/activities", s.requireAdmin(s.handleAdminCreateActivity))
	mux.Handle("GET /v1/admin/activities/{id}", s.requireAdmin(s.handleAdminGetActivity))
	mux.Handle("PUT /v1/admin/activities/{id}", s.requireAdmin(s.handleAdminUpdateActivity))
	mux.Handle("DELETE /v1/admin/activities/{id}", s.requireAdmin(s.handleAdminArchiveActivity))
	mux.Handle("GET /v1/admin/activities/{id}/participants", s.requireAdmin(s.handleAdminParticipants))
	mux.Handle("GET /v1/admin/activities/{id}/winners", s.requireAdmin(s.handleAdminWinners))
	mux.Handle("POST /v1/admin/activities/{id}/draw", s.requireAdmin(s.handleAdminDraw))
	mux.Handle("POST /v1/admin/activities/{id}/close", s.requireAdmin(s.handleAdminCloseActivity))
	mux.Handle("POST /v1/admin/activities/{id}/fulfill", s.requireAdmin(s.handleAdminFulfill))
	mux.Handle("GET /v1/admin/settings", s.requireAdmin(s.handleAdminGetSettings))
	mux.Handle("PUT /v1/admin/settings", s.requireAdmin(s.handleAdminSaveSettings))
	mux.Handle("GET /v1/admin/users/search", s.requireAdmin(s.handleAdminUserSearch))
	mux.Handle("GET /v1/admin/daily-configs", s.requireAdmin(s.handleAdminListDailyConfigs))
	mux.Handle("POST /v1/admin/daily-configs", s.requireAdmin(s.handleAdminCreateDailyConfig))
	mux.Handle("PUT /v1/admin/daily-configs/{id}", s.requireAdmin(s.handleAdminUpdateDailyConfig))
	mux.Handle("PUT /v1/admin/daily-configs/{id}/enabled", s.requireAdmin(s.handleAdminSetDailyEnabled))
	mux.Handle("DELETE /v1/admin/daily-configs/{id}", s.requireAdmin(s.handleAdminDeleteDailyConfig))

	// 任务模块（管理侧）。
	mux.Handle("GET /v1/admin/tasks", s.requireAdmin(s.handleAdminListTasks))
	mux.Handle("POST /v1/admin/tasks", s.requireAdmin(s.handleAdminCreateTask))
	mux.Handle("GET /v1/admin/tasks/{id}", s.requireAdmin(s.handleAdminGetTask))
	mux.Handle("PUT /v1/admin/tasks/{id}", s.requireAdmin(s.handleAdminUpdateTask))
	mux.Handle("DELETE /v1/admin/tasks/{id}", s.requireAdmin(s.handleAdminDeleteTask))
	mux.Handle("GET /v1/admin/tasks/{id}/rewards", s.requireAdmin(s.handleAdminTaskRewards))
	mux.Handle("POST /v1/admin/tasks/{id}/fulfill", s.requireAdmin(s.handleAdminTaskFulfill))
	mux.Handle("GET /v1/admin/tasks/{id}/codes", s.requireAdmin(s.handleAdminTaskCodesGet))
	mux.Handle("PUT /v1/admin/tasks/{id}/codes", s.requireAdmin(s.handleAdminTaskCodesPut))
	mux.Handle("GET /v1/admin/task-options/groups", s.requireAdmin(s.handleAdminTaskGroups))
	mux.Handle("GET /v1/admin/task-options/groups/{id}/models", s.requireAdmin(s.handleAdminTaskGroupModels))
	mux.Handle("GET /v1/admin/task-settings", s.requireAdmin(s.handleAdminGetTaskSettings))
	mux.Handle("PUT /v1/admin/task-settings", s.requireAdmin(s.handleAdminSaveTaskSettings))

	return s.cors(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]any{
		"status":         "ok",
		"version":        s.Version,
		"started_at":     s.StartedAt,
		"uptime_seconds": int64(time.Since(s.StartedAt).Seconds()),
	})
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// ---- User handlers ----

func (s *Server) handleListActivities(w http.ResponseWriter, r *http.Request, claims *Claims) {
	views, err := s.App.ListUserActivities(r.Context(), claims.UserID, claims.Email, claims.Role, claims.RegisteredAt)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"activities": views})
}

func (s *Server) handleEligibility(w http.ResponseWriter, r *http.Request, claims *Claims) {
	views, err := s.App.EligibilityList(r.Context(), claims.UserID, claims.Email, claims.RegisteredAt)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"activities": views})
}

// handleActivityWinners 用户侧中奖名单（邮箱脱敏）。
func (s *Server) handleActivityWinners(w http.ResponseWriter, r *http.Request, claims *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	// 用户侧显隐：部分人可见模式下，白名单外用户不返回任何中奖数据。
	// 未满足参与条件、且未参与/未中奖的场次同样不返回名单。
	if !s.App.IsUserAllowed(claims.Role, claims.Email) {
		ok(w, map[string]any{"winners": []lottery.Winner{}})
		return
	}
	canSee, err := s.App.UserCanSeeActivity(r.Context(), id, claims.UserID, claims.Email, claims.Role, claims.RegisteredAt)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	if !canSee {
		ok(w, map[string]any{"winners": []lottery.Winner{}})
		return
	}
	a, err := s.App.Store.GetActivity(id)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	if a.DrawnAt.IsZero() {
		ok(w, map[string]any{"winners": []lottery.Winner{}})
		return
	}
	winners, err := s.App.AdminWinners(id)
	if err != nil {
		internalError(w, err)
		return
	}
	// 用户侧名单对外展示：邮箱一律脱敏（管理端才看完整邮箱）。
	for i := range winners {
		winners[i].Email = lottery.MaskEmail(winners[i].Email)
	}
	ok(w, map[string]any{"winners": winners})
}

// handleMyVisibility 用户侧显隐结果 + 当前抽奖状态（供侧边栏入口显隐与状态角标）。
// 角标只反映该用户可参与（或已参与/已中奖）的场次。
func (s *Server) handleMyVisibility(w http.ResponseWriter, r *http.Request, claims *Claims) {
	visible := s.App.IsUserAllowed(claims.Role, claims.Email)
	phase := ""
	if visible {
		phase = s.App.UserLotteryPhase(r.Context(), claims.UserID, claims.Email, claims.Role, claims.RegisteredAt)
	}
	ok(w, map[string]any{"visible": visible, "phase": phase})
}

func (s *Server) handleMyWinnings(w http.ResponseWriter, r *http.Request, claims *Claims) {
	winners, err := s.App.MyWinnings(claims.UserID)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"winners": winners})
}

// handleParticipate 参与抽奖：身份经 /auth/me 权威复核后登记。
func (s *Server) handleParticipate(w http.ResponseWriter, r *http.Request, claims *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		fail(w, http.StatusUnauthorized, 401, "missing bearer token")
		return
	}
	identity, err := s.App.Sub2API.Introspect(r.Context(), strings.TrimPrefix(h, "Bearer "))
	if err != nil {
		slog.Warn("lotteryd introspect failed", "err", err)
		fail(w, http.StatusUnauthorized, 401, "identity verification failed")
		return
	}
	view, err := s.App.Participate(r.Context(), id, identity.UserID, identity, time.Now())
	if err != nil {
		mapStoreError(w, err)
		return
	}
	ok(w, view)
}

func mapStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, 404, "activity not found")
	case errors.Is(err, store.ErrAlreadyJoined):
		fail(w, http.StatusConflict, 409, "already joined")
	case errors.Is(err, store.ErrActivityFull):
		fail(w, http.StatusConflict, 409, "activity is full")
	case errors.Is(err, store.ErrNotJoinable):
		fail(w, http.StatusConflict, 409, "activity is not open for participation")
	case errors.Is(err, store.ErrNotEligible):
		fail(w, http.StatusForbidden, 403, "conditions not met")
	case errors.Is(err, store.ErrNotVisible):
		fail(w, http.StatusForbidden, 403, "lottery is not available to this user")
	case errors.Is(err, store.ErrRepeatJoin):
		fail(w, http.StatusForbidden, 403, "您已参与过该系列抽奖，不能重复参与")
	case errors.Is(err, store.ErrRepeatWin):
		fail(w, http.StatusForbidden, 403, "您已在该系列抽奖中中奖，不能再参与")
	case errors.Is(err, store.ErrActivityDrawn):
		fail(w, http.StatusConflict, 409, "activity already drawn")
	default:
		internalError(w, err)
	}
}

func internalError(w http.ResponseWriter, err error) {
	slog.Error("lotteryd request failed", "err", err)
	fail(w, http.StatusInternalServerError, 500, "internal error")
}

// ---- Admin handlers ----

type activityInput struct {
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	StartsAt             time.Time              `json:"starts_at"`
	DrawsAt              time.Time              `json:"draws_at"`
	MaxParticipants      int64                  `json:"max_participants"`
	ShowParticipantCount bool                   `json:"show_participant_count"`
	ConditionMatch       string                 `json:"condition_match"`
	AutoBonusPercent     float64                `json:"auto_bonus_percent"`
	DailyConfigID        int64                  `json:"daily_config_id"`
	Conditions           []lottery.ConditionDef `json:"conditions"`
	Prizes               []prizeInput           `json:"prizes"`
}

type prizeInput struct {
	Name      string   `json:"name"`
	PrizeType string   `json:"prize_type"`
	Value     float64  `json:"value"`
	Weight    float64  `json:"weight"`
	Stock     int64    `json:"stock"`
	Codes     []string `json:"codes"`
}

func (in *activityInput) toActivity() *lottery.Activity {
	a := &lottery.Activity{
		Name:                 in.Name,
		Description:          in.Description,
		StartsAt:             in.StartsAt,
		DrawsAt:              in.DrawsAt,
		MaxParticipants:      in.MaxParticipants,
		ShowParticipantCount: in.ShowParticipantCount,
		ConditionMatch:       in.ConditionMatch,
		AutoBonusPercent:     in.AutoBonusPercent,
		DailyConfigID:        in.DailyConfigID,
		Conditions:           in.Conditions,
	}
	for _, p := range in.Prizes {
		a.Prizes = append(a.Prizes, lottery.Prize{
			Name: p.Name, PrizeType: p.PrizeType, Value: p.Value, Weight: p.Weight, Stock: p.Stock,
			Codes: p.Codes,
		})
	}
	if a.ConditionMatch == "" {
		a.ConditionMatch = lottery.MatchAll
	}
	return a
}

func (s *Server) handleAdminCreateActivity(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var in activityInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	a := in.toActivity()
	if err := a.Validate(); err != nil {
		fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	id, err := s.App.Store.CreateActivity(a)
	if err != nil {
		internalError(w, err)
		return
	}
	s.wsHub.Wake()
	ok(w, map[string]any{"id": id})
}

func (s *Server) handleAdminUpdateActivity(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	var in activityInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	a := in.toActivity()
	a.ID = id
	if err := a.Validate(); err != nil {
		fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if err := s.App.Store.UpdateActivity(a); err != nil {
		mapStoreError(w, err)
		return
	}
	s.wsHub.Wake()
	ok(w, map[string]any{"id": id})
}

func (s *Server) handleAdminArchiveActivity(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	if err := s.App.Store.SetActivityStatus(id, "archived"); err != nil {
		mapStoreError(w, err)
		return
	}
	ok(w, map[string]any{"id": id, "status": "archived"})
}

func (s *Server) handleAdminListActivities(w http.ResponseWriter, r *http.Request, _ *Claims) {
	views, err := s.App.AdminListActivities(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"activities": views})
}

// handleAdminGetActivity 管理端单活动详情：含条件定义（供编辑）与脱敏中奖名单。
func (s *Server) handleAdminGetActivity(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	a, err := s.App.Store.GetActivity(id)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	winners, err := s.App.AdminWinners(id)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"activity": a, "winners": winners})
}

func (s *Server) handleAdminParticipants(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	list, err := s.App.AdminParticipants(id)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"participants": list})
}

func (s *Server) handleAdminWinners(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	list, err := s.App.AdminWinners(id)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"winners": list})
}

// handleAdminGetSettings 设置弹窗数据（密钥脱敏展示 + 显隐配置）。
func (s *Server) handleAdminGetSettings(w http.ResponseWriter, r *http.Request, _ *Claims) {
	vis := s.App.GetVisibility()
	ok(w, map[string]any{
		"sub2api_url":          s.App.Sub2API.BaseURL(),
		"admin_api_key_set":    s.App.Sub2API.AdminAPIKey() != "",
		"admin_api_key_masked": lottery.MaskSecret(s.App.Sub2API.AdminAPIKey()),
		"visibility":           vis,
	})
}

// handleAdminSaveSettings 保存管理员 API Key，立即生效。
func (s *Server) handleAdminSaveSettings(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var body struct {
		AdminAPIKey string              `json:"admin_api_key"`
		Visibility  *lottery.Visibility `json:"visibility,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	// Key 留空 = 不修改（只更新显隐等配置）
	key := strings.TrimSpace(body.AdminAPIKey)
	if key != "" {
		if err := s.App.SetAdminAPIKey(key); err != nil {
			internalError(w, err)
			return
		}
	}
	// 显隐配置可选一并保存
	if body.Visibility != nil {
		if err := s.App.SetVisibility(*body.Visibility); err != nil {
			internalError(w, err)
			return
		}
	}
	ok(w, map[string]any{"saved": true, "masked": lottery.MaskSecret(key)})
}

// handleAdminUserSearch 搜索用户（白名单选择用，代理主服务用户接口）。
func (s *Server) handleAdminUserSearch(w http.ResponseWriter, r *http.Request, _ *Claims) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	users, err := s.App.Sub2API.SearchUsers(r.Context(), q, limit)
	if err != nil {
		// 管理端专用接口：透传上游错误（如 401 INVALID_ADMIN_KEY），便于排查 Key/上游配置问题。
		slog.Error("lotteryd user search failed", "err", err)
		fail(w, http.StatusBadGateway, 502, "上游主服务错误: "+err.Error())
		return
	}
	ok(w, map[string]any{"users": users})
}

// handleAdminListDailyConfigs 列出全部日常定时抽奖配置。
func (s *Server) handleAdminListDailyConfigs(w http.ResponseWriter, r *http.Request, _ *Claims) {
	ok(w, map[string]any{"configs": s.App.GetDailyConfigs()})
}

// handleAdminCreateDailyConfig 新建日常定时抽奖配置。
func (s *Server) handleAdminCreateDailyConfig(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var body struct {
		Config lottery.DailyConfig `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	cfg, err := s.App.SaveDailyConfig(body.Config)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	ok(w, map[string]any{"config": cfg})
}

// handleAdminUpdateDailyConfig 更新日常定时抽奖配置（全量）。
// regenerate=true 时保存后归档当前未开奖场次并按新配置立即重建（有人参与则 409）。
func (s *Server) handleAdminUpdateDailyConfig(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid config id")
		return
	}
	var body struct {
		Config     lottery.DailyConfig `json:"config"`
		Regenerate bool                `json:"regenerate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	body.Config.ID = id
	cfg, err := s.App.SaveDailyConfig(body.Config)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	if body.Regenerate {
		if err := s.App.RegenerateDailyRound(id, time.Now()); err != nil {
			if errors.Is(err, app.ErrRegenerateBlocked) {
				fail(w, http.StatusConflict, 409, err.Error())
				return
			}
			internalError(w, err)
			return
		}
		ok(w, map[string]any{"config": cfg, "regenerated": true})
		return
	}
	ok(w, map[string]any{"config": cfg})
}

// handleAdminSetDailyEnabled 启用/停用单个配置。
func (s *Server) handleAdminSetDailyEnabled(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid config id")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid body: "+err.Error())
		return
	}
	cfg, err := s.App.SetDailyConfigEnabled(id, body.Enabled)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	s.wsHub.Wake()
	ok(w, map[string]any{"config": cfg})
}

// handleAdminDeleteDailyConfig 删除单个配置（已创建的场次不受影响）。
func (s *Server) handleAdminDeleteDailyConfig(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid config id")
		return
	}
	if err := s.App.DeleteDailyConfig(id); err != nil {
		mapStoreError(w, err)
		return
	}
	ok(w, map[string]any{"deleted": true})
}

func (s *Server) handleAdminDraw(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	a, err := s.App.Store.GetActivity(id)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	if !a.DrawnAt.IsZero() {
		fail(w, http.StatusConflict, 409, "activity already drawn")
		return
	}
	if a.Status == "closed" {
		fail(w, http.StatusConflict, 409, "activity is closed")
		return
	}
	if err := s.App.DrawActivity(r.Context(), a); err != nil {
		internalError(w, err)
		return
	}
	s.wsHub.Wake()
	winners, err := s.App.AdminWinners(id)
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"winners": winners})
}

// handleAdminCloseActivity 手动关闭活动：停止参与、不再自动开奖、不发奖（与开奖/归档区分）。
func (s *Server) handleAdminCloseActivity(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	a, err := s.App.Store.GetActivity(id)
	if err != nil {
		mapStoreError(w, err)
		return
	}
	if !a.DrawnAt.IsZero() {
		fail(w, http.StatusConflict, 409, "activity already drawn")
		return
	}
	if a.Status == "closed" {
		fail(w, http.StatusConflict, 409, "活动已关闭，无需重复操作")
		return
	}
	if a.Status == "archived" {
		fail(w, http.StatusConflict, 409, "活动已归档，无法关闭")
		return
	}
	if a.Status != "active" {
		fail(w, http.StatusConflict, 409, "活动当前状态不可关闭")
		return
	}
	if err := s.App.Store.SetActivityStatus(id, "closed"); err != nil {
		mapStoreError(w, err)
		return
	}
	s.wsHub.Wake()
	s.App.MarkDailySkip(id)
	ok(w, map[string]any{"id": id, "status": "closed"})
}

func (s *Server) handleAdminFulfill(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, err := pathID(r)
	if err != nil {
		fail(w, http.StatusBadRequest, 400, "invalid activity id")
		return
	}
	if err := s.App.RetryFulfillment(r.Context(), id); err != nil {
		mapStoreError(w, err)
		return
	}
	ok(w, map[string]any{"id": id})
}
