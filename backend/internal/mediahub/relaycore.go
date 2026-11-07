package mediahub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ForwardInput 是发往 mediad 的一次中转请求的全部输入。
type ForwardInput struct {
	// MediadBaseURL 是 mediad 服务地址(如 http://127.0.0.1:8090)。
	MediadBaseURL string
	// RelaySecret 是与 mediad 共享的中转签名密钥。
	RelaySecret string
	// SchemaID 决定 mediad 侧的组装规则。
	SchemaID string
	// UpstreamURL/Credential 来自 sub2api 账号配置,随请求交给 mediad。
	UpstreamURL string
	Credential  string
	// ClientPath 是客户端请求的子路径(用于 mediad 日志/审计)。
	ClientPath string
	// Body 是客户端原始请求体。
	Body []byte
	// ContentType 是客户端 Content-Type(multipart 原样传递)。
	ContentType string

	// UpstreamModel 是改写后的上游模型名。
	UpstreamModel string
	// Identity 是受信身份头。
	UserID  int64
	KeyID   int64
	GroupID int64
	// IdempotencyKey 防止换号重试重复创建付费任务。
	IdempotencyKey string
}

// ForwardResponse 是 mediad 的响应。
type ForwardResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	// Usage 从 X-Media-Usage 头解析;失败置 nil 并在 UsageError 给出原因。
	Usage      *Usage
	UsageError string
}

// Forward 把请求签名后转发给 mediad,返回原样响应。
// 超时由调用方 context 控制(同步长等待场景给足预算)。
func Forward(ctx context.Context, client *http.Client, in ForwardInput) (*ForwardResponse, error) {
	if strings.TrimSpace(in.MediadBaseURL) == "" {
		return nil, fmt.Errorf("mediahub: mediad base url is empty")
	}
	// 规范路径 = 实际请求路径(与 mediad 验签使用的 r.URL.Path 一致)。
	// schema_id 约定为 URL 安全字符([a-zA-Z0-9._-]),不做转义。
	relayPath := "/relay/" + in.SchemaID
	if in.ClientPath != "" {
		relayPath += "/" + strings.TrimLeft(in.ClientPath, "/")
	}
	target := strings.TrimRight(in.MediadBaseURL, "/") + relayPath
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := Sign(in.RelaySecret, ts, http.MethodPost, relayPath, in.Body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(in.Body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", orDefault(in.ContentType, "application/json"))
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, sig)
	req.Header.Set(HeaderUpstreamURL, in.UpstreamURL)
	req.Header.Set(HeaderCredential, in.Credential)
	req.Header.Set(HeaderUserID, strconv.FormatInt(in.UserID, 10))
	req.Header.Set(HeaderKeyID, strconv.FormatInt(in.KeyID, 10))
	if in.GroupID > 0 {
		req.Header.Set(HeaderGroupID, strconv.FormatInt(in.GroupID, 10))
	}
	if in.UpstreamModel != "" {
		req.Header.Set(HeaderModel, in.UpstreamModel)
	}
	if in.IdempotencyKey != "" {
		req.Header.Set(HeaderIdempotencyKey, in.IdempotencyKey)
	}

	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mediahub: mediad unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("mediahub: read mediad response: %w", err)
	}

	out := &ForwardResponse{StatusCode: resp.StatusCode, Header: resp.Header, Body: body}
	if raw := resp.Header.Get(HeaderUsage); raw != "" {
		var u Usage
		if err := json.Unmarshal([]byte(raw), &u); err == nil && u.Unit != "" {
			out.Usage = &u
		} else {
			out.UsageError = "invalid " + HeaderUsage + " header: " + raw
		}
	}
	if raw := resp.Header.Get(HeaderUsageError); raw != "" {
		out.UsageError = raw
	}
	return out, nil
}

// ForwardTaskStatus 以签名 GET 轮询 mediad 任务状态
// (GET /media/tasks/{task_id} → mediad GET /relay/tasks/{task_id})。
// 任务归属由 mediad 按身份头强制校验,越权与未知一律 404。
func ForwardTaskStatus(ctx context.Context, client *http.Client, in ForwardInput, taskID string) (*ForwardResponse, error) {
	if strings.TrimSpace(in.MediadBaseURL) == "" {
		return nil, fmt.Errorf("mediahub: mediad base url is empty")
	}
	relayPath := "/relay/tasks/" + taskID
	target := strings.TrimRight(in.MediadBaseURL, "/") + relayPath
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := Sign(in.RelaySecret, ts, http.MethodGet, relayPath, nil)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, sig)
	req.Header.Set(HeaderUserID, strconv.FormatInt(in.UserID, 10))
	req.Header.Set(HeaderKeyID, strconv.FormatInt(in.KeyID, 10))
	if in.GroupID > 0 {
		req.Header.Set(HeaderGroupID, strconv.FormatInt(in.GroupID, 10))
	}
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mediahub: mediad unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("mediahub: read mediad response: %w", err)
	}
	return &ForwardResponse{StatusCode: resp.StatusCode, Header: resp.Header, Body: body}, nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
