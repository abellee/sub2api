// Package sub2api is the HTTP client lotteryd uses to talk to the running
// Sub2API service: pulling per-user usage data, minting redeem codes,
// crediting balances and introspecting user identity.
package sub2api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"	
	"sync/atomic"
	"time"

	"lotteryd/internal/lottery"
)

// Client talks to a Sub2API instance.
type Client struct {
	baseURL     string
	adminAPIKey atomic.Value // string，运行时可改（管理页设置）
	http        *http.Client
}

// NewClient creates a client. baseURL like http://127.0.0.1:8080.
func NewClient(baseURL, adminAPIKey string) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	c.adminAPIKey.Store(adminAPIKey)
	return c
}

// SetAdminAPIKey updates the admin API key at runtime.
func (c *Client) SetAdminAPIKey(key string) { c.adminAPIKey.Store(key) }

// AdminAPIKey returns the current admin API key.
func (c *Client) AdminAPIKey() string {
	if v := c.adminAPIKey.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// BaseURL returns the configured Sub2API base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// envelope mirrors Sub2API's {code, message, data} response wrapper.
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any, bearer string) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path+"?"+query.Encode(), reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	// Sub2API admin middleware accepts a static API key on /admin routes.
	if key := c.AdminAPIKey(); key != "" && strings.HasPrefix(path, "/api/v1/admin/") {
		req.Header.Set("x-api-key", key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sub2api %s %s: HTTP %d: %s", method, path, resp.StatusCode, truncate(string(raw), 300))
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("sub2api %s %s: decode envelope: %w", method, path, err)
	}
	if env.Code != 0 && env.Code != 200 {
		return fmt.Errorf("sub2api %s %s: code %d: %s", method, path, env.Code, env.Message)
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("sub2api %s %s: decode data: %w", method, path, err)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ---- Usage data ----

type breakdownUser struct {
	UserID      int64   `json:"user_id"`
	Email       string  `json:"email"`
	TotalTokens float64 `json:"total_tokens"`
}

type breakdownData struct {
	Users []breakdownUser `json:"users"`
}

// FetchDailyTokens pulls per-user total token usage for one day (YYYY-MM-DD,
// interpreted in the Sub2API server timezone) via the dashboard user-breakdown
// endpoint with limit=0 (all users).
func (c *Client) FetchDailyTokens(ctx context.Context, date string) ([]lottery.DailyToken, error) {
	q := url.Values{}
	q.Set("start_date", date)
	q.Set("end_date", date)
	q.Set("limit", "0")
	var data breakdownData
	if err := c.do(ctx, http.MethodGet, "/api/v1/admin/dashboard/user-breakdown", q, nil, &data, ""); err != nil {
		return nil, err
	}
	out := make([]lottery.DailyToken, 0, len(data.Users))
	for _, u := range data.Users {
		out = append(out, lottery.DailyToken{Date: date, UserID: u.UserID, Email: u.Email, TotalTokens: u.TotalTokens})
	}
	return out, nil
}

// ---- 分组/模型与按口径用量（复用 Sub2API 原生管理接口，无专用集成端点） ----

// FetchDailyModelUsage 拉取某日（YYYY-MM-DD，Sub2API 服务器时区）按分组+模型
// 过滤的每用户 token 消耗。groupID<=0 表示全部分组，model 为空表示不限模型。
// 复用原生 dashboard user-breakdown 接口（其本身支持 model/group_id 过滤）。
func (c *Client) FetchDailyModelUsage(ctx context.Context, date string, groupID int64, model string) ([]lottery.DailyToken, error) {
	q := url.Values{}
	q.Set("start_date", date)
	q.Set("end_date", date)
	q.Set("limit", "0")
	if groupID > 0 {
		q.Set("group_id", strconv.FormatInt(groupID, 10))
	}
	if model != "" {
		q.Set("model", model)
	}
	var data breakdownData
	if err := c.do(ctx, http.MethodGet, "/api/v1/admin/dashboard/user-breakdown", q, nil, &data, ""); err != nil {
		return nil, err
	}
	out := make([]lottery.DailyToken, 0, len(data.Users))
	for _, u := range data.Users {
		out = append(out, lottery.DailyToken{Date: date, UserID: u.UserID, Email: u.Email, TotalTokens: u.TotalTokens})
	}
	return out, nil
}

// IntegrationGroup 原生分组接口返回的分组摘要（GET /admin/groups/all）。
type IntegrationGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
}

// ListIntegrationGroups 列出全部分组（原生 groups/all，返回裸数组）。
func (c *Client) ListIntegrationGroups(ctx context.Context) ([]IntegrationGroup, error) {
	var groups []IntegrationGroup
	if err := c.do(ctx, http.MethodGet, "/api/v1/admin/groups/all", nil, nil, &groups, ""); err != nil {
		return nil, err
	}
	return groups, nil
}

// ListGroupModels 列出分组内可选模型（原生 model-allowlist-candidates 接口）。
func (c *Client) ListGroupModels(ctx context.Context, groupID int64) ([]string, error) {
	var data struct {
		Models []string `json:"models"`
	}
	path := "/api/v1/admin/groups/" + strconv.FormatInt(groupID, 10) + "/model-allowlist-candidates"
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &data, ""); err != nil {
		return nil, err
	}
	return data.Models, nil
}

// ---- Prize fulfillment ----

type generatedCode struct {
	Code string `json:"code"`
}

type generateResult struct {
	Items []generatedCode `json:"items"`
}

// GenerateRedeemCode mints one balance redeem code of the given value and
// returns its code string. The strings are returned directly by the generate
// endpoint, no list call needed.
func (c *Client) GenerateRedeemCode(ctx context.Context, value float64) (string, error) {
	body := map[string]any{
		"count": 1,
		"type":  "balance",
		"value": value,
	}
	var data generateResult
	if err := c.do(ctx, http.MethodPost, "/api/v1/admin/redeem-codes/generate", nil, body, &data, ""); err != nil {
		return "", err
	}
	// Response shape tolerance: either {items:[{code}]} or a bare array.
	if len(data.Items) == 0 {
		var arr []generatedCode
		if err := c.do(ctx, http.MethodPost, "/api/v1/admin/redeem-codes/generate", nil, body, &arr, ""); err == nil && len(arr) > 0 {
			return arr[0].Code, nil
		}
		return "", fmt.Errorf("sub2api generate: no code returned")
	}
	return data.Items[0].Code, nil
}

// AddBalance credits a user's balance with an admin-visible-and-user-visible
// note. amount must be > 0.
func (c *Client) AddBalance(ctx context.Context, userID int64, amount float64, notes string) error {
	body := map[string]any{
		"operation": "add",
		"balance":   amount,
		"notes":     notes,
	}
	path := "/api/v1/admin/users/" + strconv.FormatInt(userID, 10) + "/balance"
	return c.do(ctx, http.MethodPost, path, nil, body, nil, "")
}

// ---- Identity ----

// Identity is the authenticated user resolved from a Bearer token.
type Identity struct {
	UserID int64  `json:"id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	// RegisteredAt 账号注册时间（/auth/me 响应里的 created_at；解析失败为 nil）。
	RegisteredAt *time.Time `json:"-"`
}

// Introspect resolves the identity behind a user JWT by calling
// GET /api/v1/auth/me with the same token. This is the authoritative check
// (catches revoked token versions that local validation cannot see).
func (c *Client) Introspect(ctx context.Context, bearer string) (*Identity, error) {
	var raw struct {
		ID        int64  `json:"id"`
		Email     string `json:"email"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/auth/me", nil, nil, &raw, bearer); err != nil {
		return nil, err
	}
	if raw.ID <= 0 {
		return nil, fmt.Errorf("sub2api auth/me: no user id")
	}
	id := &Identity{UserID: raw.ID, Email: raw.Email, Role: raw.Role}
	if raw.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.CreatedAt); err == nil {
			id.RegisteredAt = &t
		}
	}
	return id, nil
}

// SearchUser 搜索用户（管理端白名单选择用）。
type SearchUser struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// SearchUsers 按关键字搜索用户（邮箱/用户名模糊匹配），供白名单选择。
func (c *Client) SearchUsers(ctx context.Context, query string, limit int) ([]SearchUser, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchUser{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	q := url.Values{}
	q.Set("search", query)
	q.Set("page", "1")
	q.Set("page_size", strconv.Itoa(limit))
	var data struct {
		Items []SearchUser `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/admin/users", q, nil, &data, ""); err != nil {
		return nil, err
	}
	return data.Items, nil
}

// RegisteredUser 单用户详情（用于解析注册时间）。
type RegisteredUser struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	CreatedAt string `json:"created_at"`
}

// GetUser 拉取单个用户详情（含 created_at），注册时长条件的数据来源兜底。
func (c *Client) GetUser(ctx context.Context, userID int64) (*RegisteredUser, error) {
	var u RegisteredUser
	path := "/api/v1/admin/users/" + strconv.FormatInt(userID, 10)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &u, ""); err != nil {
		return nil, err
	}
	return &u, nil
}
