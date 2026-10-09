package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lotteryd/internal/app"
	"lotteryd/internal/lottery"
	"lotteryd/internal/store"
	"lotteryd/internal/sub2api"
)

// fakeSub2API 模拟主服务：用量查询、兑换码生成、余额入账、身份内省。
type fakeSub2API struct {
	srv         *httptest.Server
	dailyTokens map[string]map[int64]float64 // date -> user -> tokens
	// modelUsage date -> user -> tokens（固定口径 grok4.6@Grok-Heavy，任务集成接口用）
	modelUsage  map[string]map[int64]float64
	balances    map[int64]float64
	balanceNote string
	codes       []string
}

func newFakeSub2API(t *testing.T) *fakeSub2API {
	f := &fakeSub2API{
		dailyTokens: map[string]map[int64]float64{},
		modelUsage:  map[string]map[int64]float64{},
		balances:    map[int64]float64{},
		codes:       []string{"LOTTO-CODE-0001", "LOTTO-CODE-0002", "LOTTO-CODE-0003"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/dashboard/user-breakdown", func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("start_date")
		// 任务口径：带 model/group_id 过滤时返回 modelUsage（模拟按模型过滤后的用量）
		source := f.dailyTokens
		if r.URL.Query().Get("model") != "" || r.URL.Query().Get("group_id") != "" {
			source = f.modelUsage
		}
		users := make([]map[string]any, 0)
		for uid, tokens := range source[date] {
			users = append(users, map[string]any{
				"user_id": uid, "email": fmt.Sprintf("user%d@test.com", uid), "total_tokens": tokens,
			})
		}
		writeEnv(w, map[string]any{"users": users})
	})
	mux.HandleFunc("GET /api/v1/admin/groups/all", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, []map[string]any{{"id": 7, "name": "Grok-Heavy", "platform": "grok", "status": "active"}})
	})
	mux.HandleFunc("GET /api/v1/admin/groups/{id}/model-allowlist-candidates", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, map[string]any{"models": []string{"grok4.6", "grok4.5"}})
	})
	mux.HandleFunc("POST /api/v1/admin/redeem-codes/generate", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Value float64 `json:"value"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		code := f.codes[0]
		f.codes = f.codes[1:]
		writeEnv(w, map[string]any{"items": []map[string]any{{"code": code, "value": body.Value}}})
	})
	mux.HandleFunc("POST /api/v1/admin/users/{id}/balance", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Balance   float64 `json:"balance"`
			Operation string  `json:"operation"`
			Notes     string  `json:"notes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		require.Equal(t, "add", body.Operation)
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		f.balances[id] += body.Balance
		f.balanceNote = body.Notes
		writeEnv(w, map[string]any{"id": id})
	})
	mux.HandleFunc("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		// 从 Bearer token 的 payload 段解析身份（测试 token 无需验签）。
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		parts := strings.Split(token, ".")
		raw, err := base64.RawURLEncoding.DecodeString(parts[len(parts)-2])
		if err != nil {
			writeEnv(w, map[string]any{"id": 0})
			return
		}
		var claims struct {
			UserID float64 `json:"user_id"`
			Email  string  `json:"email"`
			Role   string  `json:"role"`
		}
		_ = json.Unmarshal(raw, &claims)
		writeEnv(w, map[string]any{"id": int64(claims.UserID), "email": claims.Email, "role": claims.Role})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func writeEnv(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "message": "", "data": data})
}

// signJWT 生成测试用 HS256 token（与 Sub2API claims 结构一致）。
func signJWT(secret string, userID int64, role string) string {
	b64 := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	header := b64(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload := b64(map[string]any{
		"user_id": float64(userID), "email": fmt.Sprintf("user%d@test.com", userID),
		"role": role, "exp": time.Now().Add(time.Hour).Unix(),
	})
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(header + "." + payload))
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

type testEnv struct {
	fake   *fakeSub2API
	srv    *httptest.Server
	app    *app.App
	secret string
}

func newTestEnv(t *testing.T) *testEnv {
	fake := newFakeSub2API(t)
	st, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	client := sub2api.NewClient(fake.srv.URL, "test-admin-key")
	ap := app.New(st, client, app.Config{BackfillDays: 35})
	srv := New(ap, "test-jwt-secret", "test")
	return &testEnv{
		fake:   fake,
		srv:    httptest.NewServer(srv.Handler()),
		app:    ap,
		secret: "test-jwt-secret",
	}
}

func mustJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Equal(t, 0, env.Code, "response should succeed: %s", env.Message)
	if len(env.Data) == 0 {
		return nil
	}
	var data map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &data))
	return data
}

func (e *testEnv) req(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	r, err := http.NewRequest(method, e.srv.URL+path, reader)
	require.NoError(t, err)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(r)
	require.NoError(t, err)
	return resp
}

// mustMenu 读取侧边栏聚合探测 /v1/me/menu 的 data。
func (e *testEnv) mustMenu(t *testing.T, token string) map[string]any {
	t.Helper()
	return mustJSON(t, e.req(t, http.MethodGet, "/v1/me/menu", token, nil))
}

const isoFmt = "2006-01-02T15:04:05Z07:00"

func sampleActivity(startsAt, drawsAt time.Time) map[string]any {
	return map[string]any{
		"name":               "Grok Heavy 七日挑战",
		"description":        "连续七天日耗 100M",
		"starts_at":          startsAt.Format(isoFmt),
		"draws_at":           drawsAt.Format(isoFmt),
		"max_participants":   0,
		"condition_match":    "all",
		"auto_bonus_percent": 25,
		"conditions": []map[string]any{
			{"dimension": "token_usage", "window_days": 7, "mode": "per_day", "threshold": 100000000, "bonus_mode": "manual", "bonus_percent": 50},
		},
		"prizes": []map[string]any{
			{"name": "余额奖", "prize_type": "balance", "value": 5, "weight": 1, "stock": 1},
			{"name": "兑换码奖", "prize_type": "redeem_code", "value": 2, "weight": 0, "stock": 2},
		},
	}
}

func TestLotteryEndToEnd(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	// 默认显隐已改为 partial（白名单空），抽奖流程测试放开为全员可见
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	userToken := signJWT(e.secret, 7, "user")
	otherToken := signJWT(e.secret, 8, "user")

	now := time.Now().UTC()
	// 1. 管理员创建活动。
	resp := e.req(t, http.MethodPost, "/v1/admin/activities", adminToken,
		sampleActivity(now.Add(-24*time.Hour), now.Add(time.Hour)))
	data := mustJSON(t, resp)
	activityID := int64(data["id"].(float64))
	require.Greater(t, activityID, int64(0))

	// 2. 喂 7 天达标用量（终点为昨天，与评估窗口一致；日期按北京时间对齐）并同步。
	localNow := time.Now().In(lottery.Beijing())
	for i := 1; i <= 8; i++ {
		date := localNow.AddDate(0, 0, -i).Format("2006-01-02")
		e.fake.dailyTokens[date] = map[int64]float64{7: 110e6, 8: 10e6}
	}
	require.NoError(t, e.app.SyncTokens(context.Background(), localNow))

	// 3. 用户查活动列表：两边都能看到这场。user7 合格（权重 1.5），user8 不合格但不能报名。
	resp = e.req(t, http.MethodGet, "/v1/activities", userToken, nil)
	data = mustJSON(t, resp)
	acts := data["activities"].([]any)
	require.Len(t, acts, 1)
	view := acts[0].(map[string]any)
	require.True(t, view["eligible"].(bool))
	// 用户侧不展示加成/权重：weight 恒为 1（内部仍按 1.5 加权抽签）。
	require.InDelta(t, 1.0, view["weight"].(float64), 1e-9)

	resp = e.req(t, http.MethodGet, "/v1/activities", otherToken, nil)
	data = mustJSON(t, resp)
	otherActs := data["activities"].([]any)
	require.Len(t, otherActs, 1, "ineligible user still sees the activity")
	require.False(t, otherActs[0].(map[string]any)["eligible"].(bool))
	vis := e.mustMenu(t, userToken)
	require.Equal(t, true, vis["lottery_visible"].(bool))
	require.Equal(t, "joining", vis["lottery_phase"].(string), "badge follows the global joining activity")
	vis = e.mustMenu(t, otherToken)
	require.Equal(t, true, vis["lottery_visible"].(bool))
	require.Equal(t, "joining", vis["lottery_phase"].(string), "ineligible user gets the same lottery badge")

	// 4. user7 参与成功；重复参与 409；不合格的 user8 参与被拒 403。
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", activityID), userToken, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", activityID), userToken, nil)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	_ = resp.Body.Close()
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", activityID), otherToken, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()

	// 5. eligibility 列表：已参与 → 为空；未参与且不合格 → 为空。
	resp = e.req(t, http.MethodGet, "/v1/eligibility", userToken, nil)
	data = mustJSON(t, resp)
	require.Empty(t, data["activities"].([]any))
	resp = e.req(t, http.MethodGet, "/v1/eligibility", otherToken, nil)
	data = mustJSON(t, resp)
	require.Empty(t, data["activities"].([]any))

	// 6. 手动开奖：唯一参与者中奖，名单邮箱脱敏。
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/draw", activityID), adminToken, nil)
	data = mustJSON(t, resp)
	winners := data["winners"].([]any)
	require.Len(t, winners, 1)
	winner := winners[0].(map[string]any)
	// 管理端为运营工具：中奖名单保留完整邮箱，便于核对与发放排查。
	require.Equal(t, "user7@test.com", winner["email"], "管理端名单邮箱不脱敏")

	// 7. 已开奖活动不可再编辑、不可重复开奖。
	resp = e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/activities/%d", activityID), adminToken,
		sampleActivity(now.Add(-24*time.Hour), now.Add(time.Hour)))
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	_ = resp.Body.Close()
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/draw", activityID), adminToken, nil)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	_ = resp.Body.Close()

	// 8. 我的中奖记录：余额奖已入账且备注=活动名+金额；兑换码奖只对本人可见。
	resp = e.req(t, http.MethodGet, "/v1/my/winnings", userToken, nil)
	data = mustJSON(t, resp)
	mine := data["winners"].([]any)
	require.Len(t, mine, 1)
	m := mine[0].(map[string]any)
	switch m["prize_type"] {
	case "balance":
		require.Equal(t, 5.0, e.fake.balances[7])
		require.Equal(t, "抽奖活动: Grok Heavy 七日挑战 +5", e.fake.balanceNote)
	case "redeem_code":
		require.NotEmpty(t, m["redeem_code"], "兑换码对中奖者本人可见")
	default:
		t.Fatalf("unexpected prize type %v", m["prize_type"])
	}

	// 9. 用户侧中奖名单：公开场次对所有登录用户展示脱敏名单。
	resp = e.req(t, http.MethodGet, fmt.Sprintf("/v1/activities/%d/winners", activityID), userToken, nil)
	data = mustJSON(t, resp)
	userWinners := data["winners"].([]any)
	require.Len(t, userWinners, 1)
	require.Equal(t, "u***@test.com", userWinners[0].(map[string]any)["email"])
	resp = e.req(t, http.MethodGet, fmt.Sprintf("/v1/activities/%d/winners", activityID), otherToken, nil)
	data = mustJSON(t, resp)
	require.Len(t, data["winners"].([]any), 1)
	require.Equal(t, "u***@test.com", data["winners"].([]any)[0].(map[string]any)["email"])

	// 10. 非管理员访问管理端 → 403。
	resp = e.req(t, http.MethodGet, "/v1/admin/activities", userToken, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestEligibilityPromptFlow(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	userToken := signJWT(e.secret, 7, "user")
	// 默认显隐已改为 partial（白名单空），资格弹窗流程测试放开为全员可见
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))

	now := time.Now().UTC()
	resp := e.req(t, http.MethodPost, "/v1/admin/activities", adminToken,
		sampleActivity(now.Add(-time.Hour), now.Add(time.Hour)))
	data := mustJSON(t, resp)
	_ = data["id"].(float64)

	localNow := time.Now().In(lottery.Beijing())
	for i := 1; i <= 8; i++ {
		date := localNow.AddDate(0, 0, -i).Format("2006-01-02")
		e.fake.dailyTokens[date] = map[int64]float64{7: 110e6}
	}
	require.NoError(t, e.app.SyncTokens(context.Background(), localNow))

	// 全局资格查询：符合条件 → 返回活动供弹窗引导。
	resp = e.req(t, http.MethodGet, "/v1/eligibility", userToken, nil)
	data = mustJSON(t, resp)
	require.Len(t, data["activities"].([]any), 1)
	_ = resp.Body.Close()

	// 不符合条件的用户拿不到弹窗数据。
	otherToken := signJWT(e.secret, 8, "user")
	resp = e.req(t, http.MethodGet, "/v1/eligibility", otherToken, nil)
	data = mustJSON(t, resp)
	require.Empty(t, data["activities"].([]any))
	_ = resp.Body.Close()
}

func TestBadgeFollowsParticipatableActivities(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	heavy := signJWT(e.secret, 7, "user")
	light := signJWT(e.secret, 8, "user")
	now := time.Now().UTC()

	joining := sampleActivity(now.Add(-time.Hour), now.Add(2*time.Hour))
	joining["name"] = "进行中高门槛"
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, joining))

	upcoming := sampleActivity(now.Add(2*time.Hour), now.Add(4*time.Hour))
	upcoming["name"] = "未开始无门槛"
	upcoming["conditions"] = []map[string]any{}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, upcoming))

	localNow := time.Now().In(lottery.Beijing())
	for i := 1; i <= 8; i++ {
		date := localNow.AddDate(0, 0, -i).Format("2006-01-02")
		e.fake.dailyTokens[date] = map[int64]float64{7: 110e6}
	}
	require.NoError(t, e.app.SyncTokens(context.Background(), localNow))

	data := mustJSON(t, e.req(t, http.MethodGet, "/v1/activities", light, nil))
	lightActs := data["activities"].([]any)
	require.Len(t, lightActs, 2, "user who misses the joining condition still sees both rounds")
	vis := e.mustMenu(t, light)
	require.Equal(t, "joining", vis["lottery_phase"].(string), "badge follows the global joining round")

	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/activities", heavy, nil))
	require.Len(t, data["activities"].([]any), 2, "eligible user still sees both rounds")
	vis = e.mustMenu(t, heavy)
	require.Equal(t, "joining", vis["lottery_phase"].(string))
}

func TestAdminSeesIneligibleActivitiesAndTasks(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	user := signJWT(e.secret, 8, "user")
	now := time.Now().UTC()
	today := time.Now().In(lottery.Beijing()).Format("2006-01-02")

	joining := sampleActivity(now.Add(-time.Hour), now.Add(2*time.Hour))
	joining["name"] = "管理员也该看到"
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, joining))

	task := sampleTask("仅名单内任务", today, 7, "balance")
	task["whitelist"] = []string{"user7@test.com"}
	task["conditions"] = []map[string]any{
		{"dimension": "token_usage", "window_days": 7, "mode": "per_day", "threshold": 100000000, "bonus_mode": "none"},
	}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, task))

	data := mustJSON(t, e.req(t, http.MethodGet, "/v1/activities", user, nil))
	require.Len(t, data["activities"].([]any), 1, "every user sees the activity")
	require.False(t, data["activities"].([]any)[0].(map[string]any)["eligible"].(bool))
	require.Equal(t, "joining", e.mustMenu(t, user)["lottery_phase"].(string))
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user, nil))
	require.Len(t, data["tasks"].([]any), 1, "every user sees the task")
	require.Equal(t, "active", e.mustMenu(t, user)["task_phase"].(string))
	prompts := mustJSON(t, e.req(t, http.MethodGet, "/v1/task-prompts", user, nil))
	require.Empty(t, prompts["tasks"].([]any), "popup still skips users outside the whitelist")
	elig := mustJSON(t, e.req(t, http.MethodGet, "/v1/eligibility", user, nil))
	require.Empty(t, elig["activities"].([]any), "lottery popup still skips ineligible users")

	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/activities", adminToken, nil))
	require.Len(t, data["activities"].([]any), 1, "admin sees activities they do not qualify for")
	require.Equal(t, "joining", e.mustMenu(t, adminToken)["lottery_phase"].(string), "admin badge counts every round")
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", adminToken, nil))
	require.Len(t, data["tasks"].([]any), 1, "admin sees tasks outside the whitelist and conditions")
	require.Equal(t, "active", e.mustMenu(t, adminToken)["task_phase"].(string))
}

func TestTaskConditionsHideAndBadge(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	user7 := signJWT(e.secret, 7, "user")
	user8 := signJWT(e.secret, 8, "user")
	today := time.Now().In(lottery.Beijing()).Format("2006-01-02")

	open := sampleTask("无条件任务", today, 7, "balance")
	data := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, open))
	openID := int64(data["id"].(float64))

	cond := sampleTask("条件任务", today, 7, "balance")
	cond["conditions"] = []map[string]any{
		{"dimension": "token_usage", "window_days": 7, "mode": "per_day", "threshold": 100000000, "bonus_mode": "none"},
	}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, cond))

	names := func(token string) []string {
		t.Helper()
		data := mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", token, nil))
		raw := data["tasks"].([]any)
		out := make([]string, 0, len(raw))
		for _, row := range raw {
			out = append(out, row.(map[string]any)["name"].(string))
		}
		return out
	}
	phase := func(token string) string {
		t.Helper()
		return e.mustMenu(t, token)["task_phase"].(string)
	}

	require.ElementsMatch(t, []string{"无条件任务", "条件任务"}, names(user8), "conditional task stays visible before the user qualifies")
	require.Equal(t, "active", phase(user8), "badge stays active while any task is running")
	require.ElementsMatch(t, []string{"无条件任务", "条件任务"}, names(user7))
	prompts := promptNames(t, e, user8)
	require.Contains(t, prompts, "无条件任务")
	require.NotContains(t, prompts, "条件任务", "popup still hides the task until the user qualifies")

	open["status"] = "ended"
	mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/tasks/%d", openID), adminToken, open))
	ended := names(user8)
	require.ElementsMatch(t, []string{"无条件任务", "条件任务"}, ended, "ended tasks stay on the user list")
	for _, row := range mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user8, nil))["tasks"].([]any) {
		item := row.(map[string]any)
		if item["name"] == "无条件任务" {
			require.Equal(t, "ended", item["status"])
		}
	}
	require.Equal(t, "active", phase(user8), "badge follows the remaining active task for every user")
	require.Equal(t, "active", phase(user7))

	localNow := time.Now().In(lottery.Beijing())
	for i := 1; i <= 8; i++ {
		date := localNow.AddDate(0, 0, -i).Format("2006-01-02")
		e.fake.dailyTokens[date] = map[int64]float64{7: 110e6}
	}
	require.NoError(t, e.app.SyncTokens(context.Background(), localNow))
	require.ElementsMatch(t, []string{"无条件任务", "条件任务"}, names(user7))
	require.Equal(t, "active", phase(user7))
	require.ElementsMatch(t, []string{"无条件任务", "条件任务"}, names(user8))
	require.Equal(t, "active", phase(user8))
}

func TestHealth(t *testing.T) {
	e := newTestEnv(t)
	resp := e.req(t, http.MethodGet, "/health", "", nil)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := mustJSON(t, resp)
	require.Equal(t, "ok", data["status"].(string))
}

func TestDailyRegenerate(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	now := time.Now()

	cfg := map[string]any{
		"id": 0, "enabled": true, "repeat_policy": "unlimited",
		"start_time": "00:00", "duration_hours": 2,
		"name": "旧配置", "max_participants": 0, "condition_match": "all",
		"conditions": []map[string]any{},
		"prizes":     []map[string]any{{"name": "奖", "prize_type": "balance", "value": 1, "weight": 1, "stock": 5}},
	}
	data := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/daily-configs", adminToken, map[string]any{"config": cfg}))
	cfgID := int64(data["config"].(map[string]any)["id"].(float64))

	// 调度器预约一场未开奖场次（开启时刻已过时自动顺延到明天）
	require.NoError(t, e.app.CreateDueDailyActivity(now))
	roundID := int64(0)
	acts := mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/activities", adminToken, nil))["activities"].([]any)
	for _, a := range acts {
		am := a.(map[string]any)
		if am["status"] == "active" && am["daily_config_id"] == float64(cfgID) {
			roundID = int64(am["id"].(float64))
		}
	}
	require.NotZero(t, roundID, "scheduler should book a round for the config")

	// 有用户参与 → 重新生成被拒绝（409）
	require.NoError(t, e.app.Store.AddParticipant(roundID, 101, "user101@test.com", 1, 0))
	resp := e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/daily-configs/%d", cfgID), adminToken,
		map[string]any{"config": cfg, "regenerate": true})
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	_ = resp.Body.Close()

	// 把该场归档（模拟无参与的旧场被处理）→ 重新生成成功：旧场归档，新场用新配置
	require.NoError(t, e.app.Store.SetActivityStatus(roundID, "archived"))
	newCfg := cfg
	newCfg["name"] = "新配置"
	newCfg["conditions"] = []map[string]any{
		{"dimension": "token_usage", "window_days": 7, "mode": "per_day", "threshold": 100000000, "bonus_mode": "none"},
	}
	data = mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/daily-configs/%d", cfgID), adminToken,
		map[string]any{"config": newCfg, "regenerate": true}))
	require.Equal(t, true, data["regenerated"].(bool))

	acts = mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/activities", adminToken, nil))["activities"].([]any)
	var activeCount int
	var regeneratedName string
	var archivedCount int
	for _, a := range acts {
		am := a.(map[string]any)
		if am["daily_config_id"] != float64(cfgID) {
			continue
		}
		if am["status"] == "active" {
			activeCount++
			regeneratedName = am["name"].(string)
		}
		if am["status"] == "archived" {
			archivedCount++
		}
	}
	require.Equal(t, 1, activeCount, "exactly one active round after regeneration")
	require.Equal(t, "新配置", regeneratedName)
	require.Equal(t, 1, archivedCount, "old round archived")
}

func TestDailyStartUsesBeijing(t *testing.T) {
	e := newTestEnv(t)
	cfg, err := e.app.SaveDailyConfig(lottery.DailyConfig{
		Enabled: true, StartTime: "18:30", DurationHours: 5, Name: "晚间场",
		Prizes: []lottery.PrizeSpec{{Name: "奖", PrizeType: "balance", Value: 1, Weight: 1, Stock: 1}},
	})
	require.NoError(t, err)
	// 北京时间上午，当天 18:30 还没到，应生成当天 18:30，而不是把 18:30 当成 UTC。
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, lottery.Beijing())
	require.NoError(t, e.app.CreateDueDailyActivity(now))
	acts, err := e.app.Store.ListActivities()
	require.NoError(t, err)
	var found *lottery.Activity
	for i := range acts {
		if acts[i].DailyConfigID == cfg.ID {
			found = &acts[i]
		}
	}
	require.NotNil(t, found)
	require.Equal(t, "2026-10-08T10:30:00Z", found.StartsAt.UTC().Format(time.RFC3339))
	require.Equal(t, "2026-10-08T15:30:00Z", found.DrawsAt.UTC().Format(time.RFC3339))
}

func TestRepeatGroupSharesParticipation(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	user7 := signJWT(e.secret, 7, "user")
	user8 := signJWT(e.secret, 8, "user")
	user103 := signJWT(e.secret, 103, "user")

	morning := saveRepeatConfig(t, e, "早场", lottery.RepeatJoinOnce, "  新用户注册礼  ")
	require.Equal(t, "新用户注册礼", morning.RepeatGroup)
	afternoon := saveRepeatConfig(t, e, "午场", lottery.RepeatJoinOnce, "新用户注册礼")
	other := saveRepeatConfig(t, e, "别的礼", lottery.RepeatJoinOnce, "别的礼")
	alone := saveRepeatConfig(t, e, "单独系列", lottery.RepeatJoinOnce, "")

	morningID := openRepeatActivity(t, e, adminToken, "早场", morning.ID)
	afternoonID := openRepeatActivity(t, e, adminToken, "午场", afternoon.ID)
	otherID := openRepeatActivity(t, e, adminToken, "别的礼", other.ID)
	aloneID := openRepeatActivity(t, e, adminToken, "单独系列", alone.ID)

	stamped, err := e.app.Store.GetActivity(morningID)
	require.NoError(t, err)
	require.Equal(t, "新用户注册礼", stamped.RepeatGroup)

	resp := e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", morningID), user7, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	noon := activityView(t, e, user7, "午场")
	require.NotNil(t, noon, "repeat-blocked round stays on the public list")
	require.False(t, noon["eligible"].(bool))
	elig := mustJSON(t, e.req(t, http.MethodGet, "/v1/eligibility", user7, nil))
	for _, row := range elig["activities"].([]any) {
		require.NotEqual(t, "午场", row.(map[string]any)["name"])
	}
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", afternoonID), user7, nil)
	body := readResp(t, resp)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Contains(t, body, "您已参与过该系列抽奖")

	require.Contains(t, activityNames(t, e, user8), "午场")
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", afternoonID), user8, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", otherID), user7, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", aloneID), user7, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	// 升级前生成的场次没有组名，仍按当前配置的组一起统计。
	legacy, err := e.app.Store.CreateActivity(&lottery.Activity{
		Name: "早场旧场次", StartsAt: time.Now().Add(-time.Hour), DrawsAt: time.Now().Add(2 * time.Hour),
		ConditionMatch: lottery.MatchAll, DailyConfigID: morning.ID,
		Prizes: []lottery.Prize{{Name: "奖", PrizeType: lottery.PrizeBalance, Value: 1, Weight: 1, Stock: 5}},
	})
	require.NoError(t, err)
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", legacy), user103, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", afternoonID), user103, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()

	winA := saveRepeatConfig(t, e, "中奖早场", lottery.RepeatWinOnce, "中奖组")
	winB := saveRepeatConfig(t, e, "中奖午场", lottery.RepeatWinOnce, "中奖组")
	winAID := openRepeatActivity(t, e, adminToken, "中奖早场", winA.ID)
	winBID := openRepeatActivity(t, e, adminToken, "中奖午场", winB.ID)
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", winAID), user7, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/draw", winAID), adminToken, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", winBID), user7, nil)
	body = readResp(t, resp)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Contains(t, body, "您已在该系列抽奖中中奖")
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", winBID), user8, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	sched := saveRepeatConfig(t, e, "晚场", lottery.RepeatJoinOnce, "新用户注册礼")
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, lottery.Beijing())
	require.NoError(t, e.app.CreateDueDailyActivity(now))
	acts, err := e.app.Store.ListActivities()
	require.NoError(t, err)
	var scheduled *lottery.Activity
	for i := range acts {
		if acts[i].DailyConfigID == sched.ID {
			scheduled = &acts[i]
		}
	}
	require.NotNil(t, scheduled)
	require.Equal(t, "新用户注册礼", scheduled.RepeatGroup)

	longName := strings.Repeat("组", 65)
	resp = e.req(t, http.MethodPost, "/v1/admin/daily-configs", adminToken, map[string]any{"config": map[string]any{
		"enabled": false, "repeat_policy": "join_once", "repeat_group": longName,
		"start_time": "08:00", "duration_hours": 2, "name": "过长",
		"prizes": []map[string]any{{"name": "奖", "prize_type": "balance", "value": 1, "weight": 1, "stock": 1}},
	}})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
}

func saveRepeatConfig(t *testing.T, e *testEnv, name, policy, group string) lottery.DailyConfig {
	t.Helper()
	cfg, err := e.app.SaveDailyConfig(lottery.DailyConfig{
		Enabled: true, RepeatPolicy: policy, RepeatGroup: group,
		StartTime: "18:30", DurationHours: 2, Name: name, ConditionMatch: "all",
		Prizes: []lottery.PrizeSpec{{Name: "奖", PrizeType: "balance", Value: 1, Weight: 1, Stock: 1}},
	})
	require.NoError(t, err)
	return cfg
}

func openRepeatActivity(t *testing.T, e *testEnv, adminToken, name string, cfgID int64) int64 {
	t.Helper()
	now := time.Now().UTC()
	data := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, map[string]any{
		"name": name, "starts_at": now.Add(-time.Hour).Format(isoFmt), "draws_at": now.Add(2 * time.Hour).Format(isoFmt),
		"max_participants": 0, "condition_match": "all", "daily_config_id": cfgID, "conditions": []any{},
		"prizes": []map[string]any{{"name": "奖", "prize_type": "balance", "value": 1, "weight": 1, "stock": 5}},
	}))
	return int64(data["id"].(float64))
}

func activityView(t *testing.T, e *testEnv, token, name string) map[string]any {
	t.Helper()
	for _, row := range mustJSON(t, e.req(t, http.MethodGet, "/v1/activities", token, nil))["activities"].([]any) {
		item := row.(map[string]any)
		if item["name"] == name {
			return item
		}
	}
	return nil
}

func activityNames(t *testing.T, e *testEnv, token string) []string {
	t.Helper()
	data := mustJSON(t, e.req(t, http.MethodGet, "/v1/activities", token, nil))
	raw, _ := data["activities"].([]any)
	names := make([]string, 0, len(raw))
	for _, row := range raw {
		names = append(names, row.(map[string]any)["name"].(string))
	}
	return names
}

func readResp(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

func TestManualActivitySharesRepeatGroup(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	user := signJWT(e.secret, 201, "user")
	other := signJWT(e.secret, 202, "user")

	daily := saveRepeatConfig(t, e, "午场", lottery.RepeatJoinOnce, "新用户注册礼")
	dailyID := openRepeatActivity(t, e, adminToken, "午场", daily.ID)
	manualID := openManualRepeatActivity(t, e, adminToken, "手动加场", "join_once", "新用户注册礼")
	otherManualID := openManualRepeatActivity(t, e, adminToken, "手动另一场", "join_once", " 新用户注册礼 ")

	stored, err := e.app.Store.GetActivity(manualID)
	require.NoError(t, err)
	require.Equal(t, int64(0), stored.DailyConfigID)
	require.Equal(t, lottery.RepeatJoinOnce, stored.RepeatPolicy)
	require.Equal(t, "新用户注册礼", stored.RepeatGroup)
	stored, err = e.app.Store.GetActivity(otherManualID)
	require.NoError(t, err)
	require.Equal(t, "新用户注册礼", stored.RepeatGroup)

	resp := e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", manualID), user, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", dailyID), user, nil)
	body := readResp(t, resp)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	require.Contains(t, body, "您已参与过该系列抽奖")
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", otherManualID), user, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()
	blocked := activityView(t, e, user, "手动另一场")
	require.NotNil(t, blocked)
	require.False(t, blocked["eligible"].(bool))
	dailyView := activityView(t, e, user, "午场")
	require.NotNil(t, dailyView)
	require.False(t, dailyView["eligible"].(bool))

	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", otherManualID), other, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	// 编辑定时场次不会清掉它的共用组。
	now := time.Now().UTC()
	mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/activities/%d", dailyID), adminToken, map[string]any{
		"name": "午场改名", "starts_at": now.Add(-time.Hour).Format(isoFmt), "draws_at": now.Add(2 * time.Hour).Format(isoFmt),
		"max_participants": 0, "condition_match": "all", "conditions": []any{},
		"prizes": []map[string]any{{"name": "奖", "prize_type": "balance", "value": 1, "weight": 1, "stock": 5}},
	}))
	stored, err = e.app.Store.GetActivity(dailyID)
	require.NoError(t, err)
	require.Equal(t, "午场改名", stored.Name)
	require.Equal(t, "新用户注册礼", stored.RepeatGroup)
	require.Equal(t, lottery.RepeatJoinOnce, stored.RepeatPolicy)

	mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/activities/%d", dailyID), adminToken, map[string]any{
		"name": "午场改组", "starts_at": now.Add(-time.Hour).Format(isoFmt), "draws_at": now.Add(2 * time.Hour).Format(isoFmt),
		"max_participants": 0, "condition_match": "all", "conditions": []any{},
		"repeat_policy": "win_once", "repeat_group": " 改过的组 ",
		"prizes": []map[string]any{{"name": "奖", "prize_type": "balance", "value": 1, "weight": 1, "stock": 5}},
	}))
	stored, err = e.app.Store.GetActivity(dailyID)
	require.NoError(t, err)
	require.Equal(t, "改过的组", stored.RepeatGroup)
	require.Equal(t, lottery.RepeatWinOnce, stored.RepeatPolicy)
	var saved lottery.DailyConfig
	for _, cfg := range e.app.GetDailyConfigs() {
		if cfg.ID == daily.ID {
			saved = cfg
		}
	}
	require.Equal(t, "改过的组", saved.RepeatGroup)
	require.Equal(t, lottery.RepeatWinOnce, saved.RepeatPolicy)
}

func openManualRepeatActivity(t *testing.T, e *testEnv, adminToken, name, policy, group string) int64 {
	t.Helper()
	now := time.Now().UTC()
	data := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, map[string]any{
		"name": name, "starts_at": now.Add(-time.Hour).Format(isoFmt), "draws_at": now.Add(2 * time.Hour).Format(isoFmt),
		"max_participants": 0, "condition_match": "all", "conditions": []any{},
		"repeat_policy": policy, "repeat_group": group,
		"prizes": []map[string]any{{"name": "奖", "prize_type": "balance", "value": 1, "weight": 1, "stock": 5}},
	}))
	return int64(data["id"].(float64))
}

func sampleTask(name, startDate string, durationDays int, rewardType string) map[string]any {
	return map[string]any{
		"name":             name,
		"group_id":         7,
		"group_name":       "Grok-Heavy",
		"model":            "grok4.6",
		"start_date":       startDate,
		"duration_days":    durationDays,
		"settle_time":      "00:00",
		"threshold_tokens": 100000000, // 100M
		"reward_type":      rewardType,
		"reward_value":     1,
		"repeat_policy":    "unlimited",
	}
}

func TestTaskEndToEnd(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	adminToken := signJWT(e.secret, 1, "admin")
	// 默认显隐为 partial（白名单空），测试里放开为全员可见
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	userToken := signJWT(e.secret, 101, "user")
	today := time.Now().In(lottery.Beijing())
	yesterday := today.AddDate(0, 0, -1)
	dayBefore := today.AddDate(0, 0, -2)

	// ---- 1. 余额任务：昨日 250M → 2 个单位，发 2 元；50M 不达标 ----
	e.fake.modelUsage[dayBefore.Format("2006-01-02")] = map[int64]float64{101: 250e6, 102: 50e6}
	resp := e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, sampleTask("余额任务", dayBefore.Format("2006-01-02"), 2, "balance"))
	data := mustJSON(t, resp)
	balanceTaskID := int64(data["id"].(float64))

	// 未到结算时刻（昨日结算点=今日 00:10 已过，但先用 SettleDueTasks 直接触发）
	e.app.SettleDueTasks(ctx, today)
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/tasks/"+fmt.Sprintf("%d", balanceTaskID)+"/rewards", adminToken, nil))
	require.Len(t, data["rewards"].([]any), 1, "only user 101 qualifies (102 below threshold)")
	require.Equal(t, 2.0, e.fake.balances[101], "250M / 100M = 2 units × 1元")

	// 进行中任务（未到任何结算点）：用户侧可见
	resp = e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, sampleTask("进行中任务", today.Format("2006-01-02"), 7, "balance"))
	mustJSON(t, resp)

	// 用户侧可见与奖励记录（已结束的余额任务仍展示，状态为 ended）
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", userToken, nil))
	userTasks := data["tasks"].([]any)
	require.Len(t, userTasks, 2, "ended task stays visible next to the ongoing one")
	var sawEnded bool
	for _, row := range userTasks {
		item := row.(map[string]any)
		if item["name"] == "余额任务" {
			require.Equal(t, "ended", item["status"])
			sawEnded = true
		}
	}
	require.True(t, sawEnded)
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/me/task-rewards", userToken, nil))
	require.Len(t, data["rewards"].([]any), 1)
	require.Equal(t, "active", e.mustMenu(t, userToken)["task_phase"].(string))

	// ---- 2. 重复参与策略 join_once：第 1 天发奖，第 2 天封顶 ----
	task := sampleTask("限参与一次任务", dayBefore.Format("2006-01-02"), 2, "balance")
	task["repeat_policy"] = "join_once"
	e.fake.modelUsage[yesterday.Format("2006-01-02")] = map[int64]float64{101: 150e6}
	data = mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, task))
	joinOnceID := int64(data["id"].(float64))
	e.app.SettleDueTasks(ctx, today)
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/tasks/"+fmt.Sprintf("%d", joinOnceID)+"/rewards", adminToken, nil))
	// 两个任务共用同一份口径用量：第 1 天（前天）250M → 2 单位发奖；
	// 第 2 天（昨天）150M 达标但 join_once 已封顶
	rows := data["rewards"].([]any)
	require.Len(t, rows, 1, "join_once caps at first settlement day")
	require.Equal(t, dayBefore.Format("2006-01-02"), rows[0].(map[string]any)["settle_date"].(string))
	require.Equal(t, 4.0, e.fake.balances[101], "2 (task1) + 2 (task2 day1)")

	// ---- 3. 兑换码任务：码池不足 → failed；补码后重试成功 ----
	task = sampleTask("码池任务", dayBefore.Format("2006-01-02"), 1, "redeem_code")
	data = mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, task))
	codeTaskID := int64(data["id"].(float64))
	// 只放 1 张码，但 250M 达成 2 个单位 → 不足
	data = mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/tasks/"+fmt.Sprintf("%d", codeTaskID)+"/codes", adminToken,
		map[string]any{"codes": []string{"TASK-CODE-A"}}))
	e.app.SettleDueTasks(ctx, today)
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/tasks/"+fmt.Sprintf("%d", codeTaskID)+"/rewards", adminToken, nil))
	row := data["rewards"].([]any)[0].(map[string]any)
	require.Equal(t, "failed", row["fulfillment"].(string), "pool has 1 code but 2 units needed")

	// 补码后重试
	data = mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/tasks/"+fmt.Sprintf("%d", codeTaskID)+"/codes", adminToken,
		map[string]any{"codes": []string{"TASK-CODE-A2", "TASK-CODE-B", "TASK-CODE-C"}}))
	require.Equal(t, 3.0, data["available"].(float64))
	require.NoError(t, e.app.RetryTaskFulfillment(ctx, codeTaskID))
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/tasks/"+fmt.Sprintf("%d", codeTaskID)+"/rewards", adminToken, nil))
	row = data["rewards"].([]any)[0].(map[string]any)
	require.Equal(t, "done", row["fulfillment"].(string))
	require.Len(t, row["codes"].([]any), 2, "2 codes granted for 2 units")
	// granted 计数 = 2（首次失败的扣码回滚，不占用）
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/tasks/"+fmt.Sprintf("%d", codeTaskID)+"/codes", adminToken, nil))
	require.Equal(t, 2.0, data["granted"].(float64))

	// ---- 4. 过期任务自动置为 ended ----
	data = mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken,
		sampleTask("过期任务", today.AddDate(0, 0, -5).Format("2006-01-02"), 2, "balance")))
	expiredID := int64(data["id"].(float64))
	e.app.SettleDueTasks(ctx, today)
	resp = e.req(t, http.MethodGet, "/v1/admin/tasks/"+fmt.Sprintf("%d", expiredID), adminToken, nil)
	data = mustJSON(t, resp)
	require.Equal(t, "ended", data["task"].(map[string]any)["status"].(string))

	// ---- 5. 用户侧 phase：所有任务结束后 none？ —— 码池/余额任务仍 active ----
	require.Equal(t, "active", e.mustMenu(t, userToken)["task_phase"].(string))

	// ---- 6. 分组/模型下拉代理 ----
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/task-options/groups", adminToken, nil))
	require.Len(t, data["groups"].([]any), 1)
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/task-options/groups/7/models", adminToken, nil))
	require.Len(t, data["models"].([]any), 2)

	// ---- 7. 用户 token 不能访问管理端 ----
	resp = e.req(t, http.MethodGet, "/v1/admin/tasks", userToken, nil)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestTaskVisibility(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	user101 := signJWT(e.secret, 101, "user")
	user102 := signJWT(e.secret, 102, "user")
	today := time.Now().In(lottery.Beijing()).Format("2006-01-02")

	// 白名单只限制弹窗和结算，列表两边都能看到。
	task := sampleTask("白名单任务", today, 7, "balance")
	task["whitelist"] = []string{"user101@test.com"}
	data := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, task))
	_ = data

	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user101, nil))
	require.Len(t, data["tasks"].([]any), 1)
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user102, nil))
	require.Len(t, data["tasks"].([]any), 1, "user outside whitelist still sees the task")
	require.Equal(t, "active", e.mustMenu(t, user102)["task_phase"].(string))
	require.NotContains(t, promptNames(t, e, user102), "白名单任务")
	require.Contains(t, promptNames(t, e, user101), "白名单任务")

	// 黑名单同样不隐藏列表。
	task = sampleTask("黑名单任务", today, 7, "balance")
	task["blacklist"] = []string{"user101@test.com"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, task))

	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user101, nil))
	require.Len(t, data["tasks"].([]any), 2)
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user102, nil))
	require.Len(t, data["tasks"].([]any), 2)

	// 每条任务收成 partial 且名单为空：列表隐藏。菜单仍按右上角「全部用户」。
	setTasksVisibility(t, e, lottery.Visibility{Mode: lottery.VisibilityPartial})
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user102, nil))
	require.Empty(t, data["tasks"].([]any), "item partial hides the task list outside the whitelist")
	require.Equal(t, "none", e.mustMenu(t, user102)["task_phase"].(string))
	require.Equal(t, true, e.mustMenu(t, user102)["task_visible"].(bool), "menu follows the top-right setting, which is all")
	require.Empty(t, promptNames(t, e, user102))

	setTasksVisibility(t, e, lottery.Visibility{Mode: lottery.VisibilityPartial, AllowedEmails: []string{"user102@test.com"}})
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user102, nil))
	require.Len(t, data["tasks"].([]any), 2, "whitelisted user sees the task list")
	require.Equal(t, true, e.mustMenu(t, user102)["task_visible"].(bool))
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", user101, nil))
	require.Empty(t, data["tasks"].([]any))
}

func TestTaskPromptFlow(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	user7 := signJWT(e.secret, 7, "user")
	user8 := signJWT(e.secret, 8, "user")
	today := time.Now().In(lottery.Beijing()).Format("2006-01-02")

	// 这一条任务设为 partial 且名单为空：普通用户不弹，管理员仍看得到。
	hidden := sampleTask("未开放任务", today, 7, "balance")
	hidden["cover"] = "https://example.com/cover.png"
	hidden["description"] = "说明"
	hidden["visibility"] = map[string]any{"mode": "partial"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, hidden))
	data := mustJSON(t, e.req(t, http.MethodGet, "/v1/task-prompts", user7, nil))
	require.Empty(t, data["tasks"].([]any))
	// 管理员不受这一条任务的可见范围限制，WebSocket 用同一份结果推送。
	require.Contains(t, promptNames(t, e, adminToken), "未开放任务")

	// 无条件任务：封面、名称、说明都返回。
	open := sampleTask("开放任务", today, 7, "balance")
	open["cover"] = "https://example.com/open.png"
	open["description"] = "完成每日消耗即可领奖"
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, open))
	data = mustJSON(t, e.req(t, http.MethodGet, "/v1/task-prompts", user7, nil))
	rows := data["tasks"].([]any)
	require.NotEmpty(t, rows)
	found := false
	for _, row := range rows {
		m := row.(map[string]any)
		if m["name"] == "开放任务" {
			found = true
			require.Equal(t, "https://example.com/open.png", m["cover"])
			require.Equal(t, "完成每日消耗即可领奖", m["description"])
		}
	}
	require.True(t, found)

	// 白名单外不弹；黑名单不弹。
	wl := sampleTask("仅7", today, 7, "balance")
	wl["whitelist"] = []string{"user7@test.com"}
	wl["description"] = "只给 7"
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, wl))
	bl := sampleTask("排除7", today, 7, "balance")
	bl["blacklist"] = []string{"user7@test.com"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, bl))

	names := promptNames(t, e, user7)
	require.Contains(t, names, "仅7")
	require.NotContains(t, names, "排除7")
	names8 := promptNames(t, e, user8)
	require.NotContains(t, names8, "仅7")
	require.Contains(t, names8, "排除7")

	// 可参与条件（近 7 天每日 token）不满足则不弹。
	cond := sampleTask("条件任务", today, 7, "balance")
	cond["cover"] = "data:image/png;base64,abc"
	cond["description"] = "需要连续消耗"
	cond["conditions"] = []map[string]any{
		{"dimension": "token_usage", "window_days": 7, "mode": "per_day", "threshold": 100000000, "bonus_mode": "none"},
	}
	data = mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, cond))
	condID := int64(data["id"].(float64))
	require.NotContains(t, promptNames(t, e, user7), "条件任务")

	localNow := time.Now().In(lottery.Beijing())
	for i := 1; i <= 8; i++ {
		date := localNow.AddDate(0, 0, -i).Format("2006-01-02")
		e.fake.dailyTokens[date] = map[int64]float64{7: 110e6}
	}
	require.NoError(t, e.app.SyncTokens(context.Background(), localNow))
	require.Contains(t, promptNames(t, e, user7), "条件任务")
	require.NotContains(t, promptNames(t, e, user8), "条件任务")

	// 结束后不再提示。
	cond["status"] = "ended"
	mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/tasks/%d", condID), adminToken, cond))
	require.NotContains(t, promptNames(t, e, user7), "条件任务")
}

func promptNames(t *testing.T, e *testEnv, token string) []string {
	t.Helper()
	data := mustJSON(t, e.req(t, http.MethodGet, "/v1/task-prompts", token, nil))
	raw, _ := data["tasks"].([]any)
	names := make([]string, 0, len(raw))
	for _, row := range raw {
		names = append(names, row.(map[string]any)["name"].(string))
	}
	return names
}

func TestUserActivityOmitsParticipantCount(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	userToken := signJWT(e.secret, 7, "user")

	now := time.Now().UTC()
	hidden := sampleActivity(now.Add(-time.Hour), now.Add(time.Hour))
	hidden["name"] = "隐藏人数"
	hidden["max_participants"] = 20
	hidden["show_participant_count"] = false
	hidden["conditions"] = []map[string]any{}
	data := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, hidden))
	hiddenID := int64(data["id"].(float64))

	omitted := sampleActivity(now.Add(-time.Hour), now.Add(2*time.Hour))
	omitted["name"] = "缺省不显示"
	omitted["conditions"] = []map[string]any{}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, omitted))

	shown := sampleActivity(now.Add(-time.Hour), now.Add(3*time.Hour))
	shown["name"] = "显示人数"
	shown["show_participant_count"] = true
	shown["conditions"] = []map[string]any{}
	data = mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, shown))
	shownID := int64(data["id"].(float64))

	resp := e.req(t, http.MethodGet, "/v1/activities", userToken, nil)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	var env struct {
		Data struct {
			Activities []map[string]any `json:"activities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	require.Len(t, env.Data.Activities, 3)
	for _, view := range env.Data.Activities {
		require.Contains(t, view, "max_participants")
		_, hasCount := view["participant_count"]
		switch view["name"] {
		case "隐藏人数":
			require.False(t, hasCount, "hidden activity must not send participant_count")
			require.Equal(t, false, view["show_participant_count"])
			require.Equal(t, float64(20), view["max_participants"])
		case "缺省不显示":
			require.False(t, hasCount, "omitted flag must not send participant_count")
			require.Equal(t, false, view["show_participant_count"])
		default:
			require.True(t, hasCount, "explicit show flag sends participant_count")
			require.Equal(t, true, view["show_participant_count"])
		}
	}

	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/activities/%d/participate", hiddenID), userToken, nil)
	raw, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	require.NotContains(t, string(raw), `"participant_count"`)

	detail := mustJSON(t, e.req(t, http.MethodGet, fmt.Sprintf("/v1/admin/activities/%d", shownID), adminToken, nil))
	require.Equal(t, true, detail["activity"].(map[string]any)["show_participant_count"])

	shown["show_participant_count"] = false
	mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/activities/%d", shownID), adminToken, shown))
	detail = mustJSON(t, e.req(t, http.MethodGet, fmt.Sprintf("/v1/admin/activities/%d", shownID), adminToken, nil))
	require.Equal(t, false, detail["activity"].(map[string]any)["show_participant_count"])

	shown["show_participant_count"] = true
	mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/activities/%d", shownID), adminToken, shown))
	detail = mustJSON(t, e.req(t, http.MethodGet, fmt.Sprintf("/v1/admin/activities/%d", shownID), adminToken, nil))
	require.Equal(t, true, detail["activity"].(map[string]any)["show_participant_count"])

	resp = e.req(t, http.MethodGet, "/v1/admin/activities", adminToken, nil)
	raw, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	require.Contains(t, string(raw), `"participant_count"`)

	cfgBody := lottery.DailyConfig{
		Enabled:         true,
		RepeatPolicy:    "unlimited",
		StartTime:       "18:30",
		DurationHours:   5,
		Name:            "晚间场",
		MaxParticipants: 8,
		ConditionMatch:  "all",
		Prizes:          []lottery.PrizeSpec{{Name: "奖", PrizeType: "balance", Value: 1, Stock: 1}},
	}
	cfgBody.ShowParticipantCount = true
	cfg := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/daily-configs", adminToken, map[string]any{"config": cfgBody}))
	saved := cfg["config"].(map[string]any)
	require.Equal(t, true, saved["show_participant_count"])
	cfgID := int64(saved["id"].(float64))

	listed := mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/daily-configs", adminToken, nil))
	var found bool
	for _, item := range listed["configs"].([]any) {
		row := item.(map[string]any)
		if int64(row["id"].(float64)) == cfgID {
			found = true
			require.Equal(t, true, row["show_participant_count"])
		}
	}
	require.True(t, found)

	cfgBody.ID = cfgID
	cfgBody.ShowParticipantCount = false
	updated := mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/daily-configs/%d", cfgID), adminToken, map[string]any{
		"config": cfgBody,
	}))
	require.Equal(t, false, updated["config"].(map[string]any)["show_participant_count"])

	cfgBody.ShowParticipantCount = true
	updated = mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/daily-configs/%d", cfgID), adminToken, map[string]any{
		"config": cfgBody,
	}))
	require.Equal(t, true, updated["config"].(map[string]any)["show_participant_count"])
}

func TestActivityDeleteAndTaskArchive(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	userToken := signJWT(e.secret, 7, "user")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	now := time.Now().UTC()
	today := time.Now().In(lottery.Beijing()).Format("2006-01-02")

	joining := sampleActivity(now.Add(-time.Hour), now.Add(time.Hour))
	joining["conditions"] = []map[string]any{}
	data := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, joining))
	openID := int64(data["id"].(float64))
	resp := e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/delete", openID), adminToken, map[string]any{})
	require.Equal(t, http.StatusConflict, resp.StatusCode, "joining round cannot be deleted")
	_ = resp.Body.Close()

	mustJSON(t, e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/close", openID), adminToken, nil))
	mustJSON(t, e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/delete", openID), adminToken, map[string]any{}))
	left := mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/activities", adminToken, nil))["activities"].([]any)
	require.Empty(t, left)

	drawnBody := sampleActivity(now.Add(-2*time.Hour), now.Add(-time.Minute))
	drawnBody["conditions"] = []map[string]any{}
	data = mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, drawnBody))
	drawnID := int64(data["id"].(float64))
	require.NoError(t, e.app.Store.AddParticipant(drawnID, 7, "user7@test.com", 1, 0))
	mustJSON(t, e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/draw", drawnID), adminToken, nil))
	mustJSON(t, e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/delete", drawnID), adminToken, map[string]any{}))
	kept, err := e.app.Store.GetActivity(drawnID)
	require.NoError(t, err)
	require.Equal(t, "deleted", kept.Status)
	n, err := e.app.Store.CountWinners(drawnID)
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "soft delete keeps winner rows")
	mine := mustJSON(t, e.req(t, http.MethodGet, "/v1/my/winnings", userToken, nil))["winners"].([]any)
	require.NotEmpty(t, mine, "the winner still sees their prize")
	resp = e.req(t, http.MethodGet, fmt.Sprintf("/v1/activities/%d/winners", drawnID), userToken, nil)
	data = mustJSON(t, resp)
	require.Empty(t, data["winners"].([]any), "deleted round leaves the public list")

	archivedBody := sampleActivity(now.Add(time.Hour), now.Add(2*time.Hour))
	archivedBody["conditions"] = []map[string]any{}
	data = mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, archivedBody))
	archivedID := int64(data["id"].(float64))
	mustJSON(t, e.req(t, http.MethodDelete, fmt.Sprintf("/v1/admin/activities/%d", archivedID), adminToken, nil))
	mustJSON(t, e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/activities/%d/delete", archivedID), adminToken, map[string]any{}))

	task := sampleTask("可归档任务", today, 3, "balance")
	data = mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, task))
	taskID := int64(data["id"].(float64))
	resp = e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/tasks/%d/archive", taskID), adminToken, map[string]any{})
	require.Equal(t, http.StatusConflict, resp.StatusCode, "active task cannot be archived")
	_ = resp.Body.Close()

	task["status"] = "ended"
	mustJSON(t, e.req(t, http.MethodPut, fmt.Sprintf("/v1/admin/tasks/%d", taskID), adminToken, task))
	listed := mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", userToken, nil))["tasks"].([]any)
	require.Len(t, listed, 1)
	require.Equal(t, "ended", listed[0].(map[string]any)["status"])
	require.Equal(t, "none", e.mustMenu(t, userToken)["task_phase"])

	mustJSON(t, e.req(t, http.MethodPost, fmt.Sprintf("/v1/admin/tasks/%d/archive", taskID), adminToken, map[string]any{}))
	require.Empty(t, mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", userToken, nil))["tasks"].([]any))
	adminTasks := mustJSON(t, e.req(t, http.MethodGet, "/v1/admin/tasks", adminToken, nil))["tasks"].([]any)
	require.Len(t, adminTasks, 1)
	require.Equal(t, "archived", adminTasks[0].(map[string]any)["status"])
}

func TestTaskProgressShowsTestUsage(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	adminToken := signJWT(e.secret, 1, "admin")
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	userToken := signJWT(e.secret, 101, "user")
	today := time.Now().In(lottery.Beijing())
	date := today.Format("2006-01-02")

	progressTask := sampleTask("进度任务", date, 7, "balance")
	progressTask["progress_color"] = "#10a37f"
	progressTask["progress_svg"] = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><circle cx="8" cy="8" r="4"><animate attributeName="opacity" values="1;0.2;1" dur="1s" repeatCount="indefinite"/></circle></svg>`
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, progressTask))
	require.NoError(t, e.app.Store.UpsertTaskTestUsage(date, 7, "grok4.6", 45e6))

	item := mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", userToken, nil))["tasks"].([]any)[0].(map[string]any)
	require.Equal(t, 45e6, item["progress_tokens"])
	require.Equal(t, 100e6, item["progress_cap"])
	require.EqualValues(t, 1, item["progress_units"])
	require.Equal(t, true, item["progress_ready"])
	require.Equal(t, "#10a37f", item["progress_color"], "用户侧要能拿到配置的进度条颜色")
	require.Contains(t, item["progress_svg"], "<animate", "用户侧要能拿到进度条小动画")

	e.fake.modelUsage[date] = map[int64]float64{101: 20e6}
	item = mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", userToken, nil))["tasks"].([]any)[0].(map[string]any)
	require.Equal(t, 65e6, item["progress_tokens"])

	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	data := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, sampleTask("昨日仅测试用量", yesterday, 3, "balance")))
	taskID := int64(data["id"].(float64))
	require.NoError(t, e.app.Store.UpsertTaskTestUsage(yesterday, 7, "grok4.6", 200e6))
	e.app.SettleDueTasks(ctx, today)
	rewards := mustJSON(t, e.req(t, http.MethodGet, fmt.Sprintf("/v1/admin/tasks/%d/rewards", taskID), adminToken, nil))["rewards"].([]any)
	require.Empty(t, rewards, "test usage is display-only and does not settle")
}

func setActivitiesVisibility(t *testing.T, e *testEnv, v lottery.Visibility) {
	t.Helper()
	acts, err := e.app.Store.ListActivities()
	require.NoError(t, err)
	for i := range acts {
		acts[i].Visibility = v
		require.NoError(t, e.app.Store.UpdateActivity(&acts[i]))
	}
}

func setTasksVisibility(t *testing.T, e *testEnv, v lottery.Visibility) {
	t.Helper()
	tasks, err := e.app.Store.ListTasks()
	require.NoError(t, err)
	for i := range tasks {
		tasks[i].Visibility = v
		_, err := e.app.Store.UpsertTask(&tasks[i])
		require.NoError(t, err)
	}
}

func TestEligibleAndPartialVisibility(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	user7 := signJWT(e.secret, 7, "user")
	user8 := signJWT(e.secret, 8, "user")
	now := time.Now().UTC()
	today := time.Now().In(lottery.Beijing()).Format("2006-01-02")
	// 右上角菜单设为全部用户，后面的断言测的是每一场、每一条自己的可见范围。
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))

	high := sampleActivity(now.Add(-time.Hour), now.Add(2*time.Hour))
	high["name"] = "高门槛"
	high["visibility"] = map[string]any{"mode": "eligible"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, high))
	openRound := sampleActivity(now.Add(2*time.Hour), now.Add(4*time.Hour))
	openRound["name"] = "无门槛"
	openRound["conditions"] = []map[string]any{}
	openRound["visibility"] = map[string]any{"mode": "eligible"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, openRound))

	require.Equal(t, []string{"无门槛"}, activityNames(t, e, user8))
	vis := e.mustMenu(t, user8)
	require.Equal(t, true, vis["lottery_visible"].(bool))
	require.Equal(t, "upcoming", vis["lottery_phase"].(string), "badge only counts rounds this user can see")

	localNow := time.Now().In(lottery.Beijing())
	for i := 1; i <= 8; i++ {
		date := localNow.AddDate(0, 0, -i).Format("2006-01-02")
		e.fake.dailyTokens[date] = map[int64]float64{7: 110e6}
	}
	require.NoError(t, e.app.SyncTokens(context.Background(), localNow))
	require.ElementsMatch(t, []string{"高门槛", "无门槛"}, activityNames(t, e, user7))
	vis = e.mustMenu(t, user7)
	require.Equal(t, "joining", vis["lottery_phase"].(string))
	require.ElementsMatch(t, []string{"高门槛", "无门槛"}, activityNames(t, e, adminToken))
	require.Equal(t, []string{"无门槛"}, activityNames(t, e, user8))

	cond := sampleTask("条件任务", today, 7, "balance")
	cond["conditions"] = []map[string]any{
		{"dimension": "token_usage", "window_days": 7, "mode": "per_day", "threshold": 100000000, "bonus_mode": "none"},
	}
	cond["visibility"] = map[string]any{"mode": "eligible"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, cond))
	openTask := sampleTask("无条件任务", today, 7, "balance")
	openTask["visibility"] = map[string]any{"mode": "eligible"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, openTask))
	only := sampleTask("仅7", today, 7, "balance")
	only["whitelist"] = []string{"user7@test.com"}
	only["visibility"] = map[string]any{"mode": "eligible"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, only))

	taskNames := func(token string) []string {
		t.Helper()
		data := mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", token, nil))
		raw := data["tasks"].([]any)
		out := make([]string, 0, len(raw))
		for _, row := range raw {
			out = append(out, row.(map[string]any)["name"].(string))
		}
		return out
	}
	require.ElementsMatch(t, []string{"无条件任务"}, taskNames(user8))
	require.ElementsMatch(t, []string{"条件任务", "无条件任务", "仅7"}, taskNames(user7))
	require.ElementsMatch(t, []string{"条件任务", "无条件任务", "仅7"}, taskNames(adminToken))
	require.Equal(t, true, e.mustMenu(t, user8)["task_visible"].(bool))
	require.Equal(t, "active", e.mustMenu(t, user8)["task_phase"].(string))

	setActivitiesVisibility(t, e, lottery.Visibility{Mode: lottery.VisibilityPartial, AllowedEmails: []string{"user7@test.com"}})
	require.ElementsMatch(t, []string{"高门槛", "无门槛"}, activityNames(t, e, user7))
	require.Empty(t, activityNames(t, e, user8))
	vis = e.mustMenu(t, user8)
	require.Equal(t, true, vis["lottery_visible"].(bool), "menu stays on while the top-right setting is all")
	require.Equal(t, "", vis["lottery_phase"].(string), "badge only counts rounds this user can see")

	setTasksVisibility(t, e, lottery.Visibility{Mode: lottery.VisibilityPartial})
	require.Empty(t, taskNames(user8))
	require.Equal(t, true, e.mustMenu(t, user8)["task_visible"].(bool))
	require.Equal(t, "none", e.mustMenu(t, user8)["task_phase"].(string))

	resp := e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "nope"}})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestMenuSettingsGate(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	user7 := signJWT(e.secret, 7, "user")
	user8 := signJWT(e.secret, 8, "user")
	now := time.Now().UTC()
	today := time.Now().In(lottery.Beijing()).Format("2006-01-02")

	high := sampleActivity(now.Add(-time.Hour), now.Add(2*time.Hour))
	high["name"] = "高门槛"
	high["visibility"] = map[string]any{"mode": "all"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/activities", adminToken, high))

	cond := sampleTask("条件任务", today, 7, "balance")
	cond["conditions"] = []map[string]any{
		{"dimension": "token_usage", "window_days": 7, "mode": "per_day", "threshold": 100000000, "bonus_mode": "none"},
	}
	cond["visibility"] = map[string]any{"mode": "all"}
	mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, cond))

	localNow := time.Now().In(lottery.Beijing())
	for i := 1; i <= 8; i++ {
		date := localNow.AddDate(0, 0, -i).Format("2006-01-02")
		e.fake.dailyTokens[date] = map[int64]float64{7: 110e6}
	}
	require.NoError(t, e.app.SyncTokens(context.Background(), localNow))

	taskNames := func(token string) []string {
		t.Helper()
		data := mustJSON(t, e.req(t, http.MethodGet, "/v1/tasks", token, nil))
		raw := data["tasks"].([]any)
		out := make([]string, 0, len(raw))
		for _, row := range raw {
			out = append(out, row.(map[string]any)["name"].(string))
		}
		return out
	}

	require.Equal(t, []string{"高门槛"}, activityNames(t, e, user7))
	require.Equal(t, []string{"高门槛"}, activityNames(t, e, user8))
	require.Equal(t, []string{"条件任务"}, taskNames(user7))
	require.Equal(t, []string{"条件任务"}, taskNames(user8))
	require.Equal(t, false, e.mustMenu(t, user7)["lottery_visible"].(bool))
	require.Equal(t, false, e.mustMenu(t, user8)["lottery_visible"].(bool))
	require.Equal(t, false, e.mustMenu(t, user7)["task_visible"].(bool))
	require.Equal(t, false, e.mustMenu(t, user8)["task_visible"].(bool))

	resp := e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "eligible"}})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
	require.NoError(t, e.app.Store.SetState("visibility_json", `{"mode":"eligible","allowed_emails":[]}`))
	require.Equal(t, false, e.mustMenu(t, user7)["lottery_visible"].(bool))
	require.Equal(t, []string{"高门槛"}, activityNames(t, e, user8))

	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "partial", "allowed_emails": []string{"user8@test.com"}}}))
	vis := e.mustMenu(t, user8)
	require.Equal(t, true, vis["lottery_visible"].(bool))
	require.Equal(t, "joining", vis["lottery_phase"].(string))
	vis = e.mustMenu(t, user7)
	require.Equal(t, false, vis["lottery_visible"].(bool))
	require.Equal(t, []string{"高门槛"}, activityNames(t, e, user7))

	resp = e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "eligible"}})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
	require.Equal(t, false, e.mustMenu(t, user7)["task_visible"].(bool))
	require.Equal(t, []string{"条件任务"}, taskNames(user8))

	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "partial", "allowed_emails": []string{"user8@test.com"}}}))
	require.Equal(t, true, e.mustMenu(t, user8)["task_visible"].(bool))
	require.Equal(t, false, e.mustMenu(t, user7)["task_visible"].(bool))
	require.Equal(t, []string{"条件任务"}, taskNames(user7))

	mustJSON(t, e.req(t, http.MethodPut, "/v1/admin/task-settings", adminToken,
		map[string]any{"visibility": map[string]any{"mode": "all"}}))
	setTasksVisibility(t, e, lottery.Visibility{Mode: lottery.VisibilityPartial})
	require.Empty(t, taskNames(user8))
	require.Equal(t, true, e.mustMenu(t, user8)["task_visible"].(bool))
	require.Equal(t, "none", e.mustMenu(t, user8)["task_phase"].(string))
}

// TestMenuRouteReplacesProbes 侧边栏显隐/角标统一走 /v1/me/menu，三条分散探测接口已下线。
func TestMenuRouteReplacesProbes(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	menu := e.mustMenu(t, adminToken)
	require.Equal(t, true, menu["lottery_visible"].(bool))
	require.Equal(t, true, menu["task_visible"].(bool))
	for _, path := range []string{"/v1/me/visibility", "/v1/me/tasks/phase", "/v1/me/task-visibility"} {
		resp := e.req(t, http.MethodGet, path, adminToken, nil)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode, path+" must no longer be routed")
	}
}

// TestTaskProgressSvgRoundTrip 任务管理上传的进度条小动画（SVG 源码）与进度条颜色随任务落库并原样回显，
// 清空后保存不残留；非法颜色直接 400。
func TestTaskProgressSvgRoundTrip(t *testing.T) {
	e := newTestEnv(t)
	adminToken := signJWT(e.secret, 1, "admin")
	today := time.Now().In(lottery.Beijing()).Format("2006-01-02")
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 8"><circle cx="4" cy="4" r="2"><animate attributeName="opacity" values="1;0.2;1" dur="1.2s" repeatCount="indefinite"/></circle></svg>`

	input := sampleTask("进度动画任务", today, 7, "balance")
	input["progress_svg"] = svg
	input["progress_color"] = "#22c55e"
	created := mustJSON(t, e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, input))
	id := int64(created["id"].(float64))

	adminURL := fmt.Sprintf("/v1/admin/tasks/%d", id)
	task := mustJSON(t, e.req(t, http.MethodGet, adminURL, adminToken, nil))["task"].(map[string]any)
	require.Equal(t, svg, task["progress_svg"])
	require.Equal(t, "#22c55e", task["progress_color"])

	// 非法颜色（既不是 #RGB 也不是 #RRGGBB）直接拒绝。
	bad := sampleTask("非法颜色任务", today, 7, "balance")
	bad["progress_color"] = "red"
	resp := e.req(t, http.MethodPost, "/v1/admin/tasks", adminToken, bad)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	input["progress_svg"] = ""
	input["progress_color"] = ""
	mustJSON(t, e.req(t, http.MethodPut, adminURL, adminToken, input))
	task = mustJSON(t, e.req(t, http.MethodGet, adminURL, adminToken, nil))["task"].(map[string]any)
	require.Empty(t, task["progress_svg"])
	require.Empty(t, task["progress_color"])
}
