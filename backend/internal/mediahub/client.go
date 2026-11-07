package mediahub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CallbackConfig 是注册到 mediad 的回调配置(契约同 mediad CallbackConfig)。
type CallbackConfig struct {
	CallbackURL    string `json:"callback_url"`
	CallbackSecret string `json:"callback_secret"`
	Enabled        bool   `json:"enabled"`
	MaxAttempts    int    `json:"max_attempts,omitempty"`
	BackoffBase    int    `json:"backoff_base_seconds,omitempty"`
}

// ManagementClient 调用 mediad 管理面:下发 schema、注册回调、测试、试运行。
// 管理接口挂掉只影响配置变更,不影响已在 mediad 缓存的 schema 继续服务。
type ManagementClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c *ManagementClient) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("mediahub: mediad base url is empty")
	}
	var reader io.Reader
	payload := []byte(nil)
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = buf
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, reader)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, fmt.Errorf("mediad management %s %s: status %d: %s", method, path, resp.StatusCode, truncate(data, 300))
	}
	return data, nil
}

// PutSchema 下发/更新 schema。schemaBody 为完整 schema JSON(含 id/version)。
func (c *ManagementClient) PutSchema(ctx context.Context, schemaBody json.RawMessage) (json.RawMessage, error) {
	var probe struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(schemaBody, &probe); err != nil || strings.TrimSpace(probe.ID) == "" {
		return nil, fmt.Errorf("mediahub: schema body must contain non-empty id")
	}
	return c.do(ctx, http.MethodPut, "/management/schemas/"+url.PathEscape(probe.ID), schemaBody)
}

// Request is used by the explicitly allowed main-site administration routes.
func (c *ManagementClient) Request(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return c.do(ctx, method, path, body)
}

// PutCallback 注册回调配置。
func (c *ManagementClient) PutCallback(ctx context.Context, cfg CallbackConfig) error {
	_, err := c.do(ctx, http.MethodPut, "/management/callback", cfg)
	return err
}

// TestCallback 触发 mediad 发送一次签名测试回调。
func (c *ManagementClient) TestCallback(ctx context.Context) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/management/callback/test", map[string]any{})
}

// DryRun 预演组装(不请求上游)。
func (c *ManagementClient) DryRun(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/management/dry-run", payload)
}

// Capabilities 读取 mediad 能力自描述。
func (c *ManagementClient) Capabilities(ctx context.Context) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/capabilities", nil)
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}
