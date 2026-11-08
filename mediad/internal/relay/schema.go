// Package relay 实现 mediad 的核心:按 schema 组装上游请求、
// 提取响应/任务状态/usage,以及同步等待与异步轮询回调。
package relay

import (
	"fmt"
	"net/http"
	"strings"
)

// ContractVersion 是当前实现的接口契约版本,随 /capabilities 暴露。
const ContractVersion = "2026-11-08"

// 计费单位。usage.unit 只允许这些值;价格换算永远发生在 sub2api 侧。
const (
	UnitImage  = "image"
	UnitSecond = "second"
	UnitToken  = "token"
	UnitAction = "action"
)

// Schema 描述"客户端方言 → 上游方言"的组装与提取规则。
// sub2api 通过管理接口下发并更新;媒体服务只缓存执行。
type Schema struct {
	ID              string         `json:"id"`
	Version         int            `json:"version"`
	ContractVersion string         `json:"contract_version,omitempty"`
	Mode            string         `json:"mode"` // "sync" | "async"
	Upstream        UpstreamSpec   `json:"upstream"`
	Request         RequestSpec    `json:"request"`
	Response        ResponseSpec   `json:"response"`
	TaskStates      TaskStatesSpec `json:"task_states,omitempty"`
	// Description 仅供管理界面展示。
	Description string `json:"description,omitempty"`
}

// UpstreamSpec 定义上游调用方式。path 可含 {task_id} 占位(query)。
type UpstreamSpec struct {
	Create CallSpec  `json:"create"`
	Query  *CallSpec `json:"query,omitempty"` // async 必填
}

// CallSpec 是一次上游调用的方法与路径模板。
type CallSpec struct {
	Method              string `json:"method"`
	Path                string `json:"path"`
	TimeoutSeconds      int    `json:"timeout_seconds,omitempty"`
	PollIntervalSeconds int    `json:"poll_interval_seconds,omitempty"` // 仅 query
	MaxPollSeconds      int    `json:"max_poll_seconds,omitempty"`      // 仅 query
}

// RequestSpec 定义上游请求的组装。
type RequestSpec struct {
	// IdempotencyHeader 仅在上游明确支持相同键返回同一任务时配置。
	IdempotencyHeader string `json:"idempotency_header,omitempty"`
	// Credential 把账号凭证放进指定请求头;值可含 {credential} 占位。
	// 例如 {"Authorization": "Bearer {credential}"}。
	Credential map[string]string `json:"credential,omitempty"`
	// HeaderParams 追加静态/模板请求头;值可含 {user_id} {key_id} {group_id} {model}。
	HeaderParams map[string]string `json:"header_params,omitempty"`
	// BodyMode:"passthrough"(客户端 JSON body 原样作为上游 payload)
	// 或 "mapped"(按 FieldMap 重建)。
	BodyMode string `json:"body_mode"`
	// FieldMap 仅 BodyMode=mapped 时生效:客户端字段 → 上游字段规则。
	FieldMap map[string]FieldRule `json:"field_map,omitempty"`
	// StaticFields 恒定写入上游 body 的字段。
	StaticFields map[string]any `json:"static_fields,omitempty"`
	// Multipart 把客户端 multipart 文件转为上游 JSON body 里的 data URL 数组。
	Multipart *MultipartSpec `json:"multipart,omitempty"`
}

// FieldRule 描述单个字段的重命名/转换/丢弃。
type FieldRule struct {
	To        string         `json:"to,omitempty"`        // 上游字段名;空 = 同名
	Transform string         `json:"transform,omitempty"` // 内置转换 id;空 = passthrough
	Args      map[string]any `json:"args,omitempty"`
	Drop      bool           `json:"drop,omitempty"`
}

// MultipartSpec 定义 multipart 文件 → data URL 的转换。
type MultipartSpec struct {
	// FilesField 是客户端 multipart 中的文件字段名(精确匹配,如 "image" 或 "image[]")。
	FilesField string `json:"files_field"`
	// ToBodyField 是上游 JSON body 中接收 data URL 数组的字段。
	ToBodyField string `json:"to_body_field"`
	MaxFiles    int    `json:"max_files,omitempty"` // 0 = 不限
}

// ResponseSpec 定义上游响应的提取。
type ResponseSpec struct {
	// TaskID 是异步创建响应中上游任务号的点路径,如 "data.task_id"。
	TaskID string `json:"task_id,omitempty"`
	// Status 是查询响应中状态字符串的点路径。
	Status string `json:"status,omitempty"`
	// Usage 描述计费用量。
	Usage UsageSpec `json:"usage"`
	// Result 命名结果提取:name → 点路径,如 {"image_urls": "data.result.data.image_urls"}。
	Result map[string]string `json:"result,omitempty"`
	// ErrorMessage 是失败响应中错误信息的点路径(可选)。
	ErrorMessage string `json:"error_message,omitempty"`
}

// UsageSpec 描述一次成功生成的计费用量。
// Quantity 为数字 = 固定量;为字符串 = 从上游响应按点路径提取。
// "$result_files" counts the actual result files, including Gemini inlineData images.
type UsageSpec struct {
	Quantity any    `json:"quantity"`
	Unit     string `json:"unit"`
}

// TaskStatesSpec 定义异步状态机的归类枚举。
type TaskStatesSpec struct {
	Success []string `json:"success"`
	Failure []string `json:"failure"`
	// InProgress 可省略:不在 Success/Failure 里的一律视为进行中。
	InProgress []string `json:"in_progress,omitempty"`
}

// Validate 校验 schema 的结构性约束。
func (s *Schema) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("schema.id is required")
	}
	if s.Version <= 0 {
		return fmt.Errorf("schema.version must be positive")
	}
	switch s.Mode {
	case "sync", "async":
	case "":
		return fmt.Errorf("schema.mode is required (sync|async)")
	default:
		return fmt.Errorf("schema.mode must be sync or async, got %q", s.Mode)
	}
	c := s.Upstream.Create
	if !strings.EqualFold(c.Method, http.MethodPost) && !strings.EqualFold(c.Method, http.MethodGet) {
		return fmt.Errorf("upstream.create.method must be POST or GET, got %q", c.Method)
	}
	if !strings.HasPrefix(strings.TrimSpace(c.Path), "/") {
		return fmt.Errorf("upstream.create.path must start with /")
	}
	if s.Mode == "async" {
		q := s.Upstream.Query
		if q == nil {
			return fmt.Errorf("async schema requires upstream.query")
		}
		if !strings.HasPrefix(strings.TrimSpace(q.Path), "/") || !strings.Contains(q.Path, "{task_id}") {
			return fmt.Errorf("upstream.query.path must contain {task_id}")
		}
		if s.Response.TaskID == "" {
			return fmt.Errorf("async schema requires response.task_id")
		}
		if s.Response.Status == "" {
			return fmt.Errorf("async schema requires response.status")
		}
		if len(s.TaskStates.Success) == 0 {
			return fmt.Errorf("async schema requires task_states.success")
		}
	}
	if s.Request.BodyMode != "passthrough" && s.Request.BodyMode != "mapped" {
		return fmt.Errorf("request.body_mode must be passthrough or mapped, got %q", s.Request.BodyMode)
	}
	if s.Request.Multipart != nil {
		m := s.Request.Multipart
		if strings.TrimSpace(m.FilesField) == "" || strings.TrimSpace(m.ToBodyField) == "" {
			return fmt.Errorf("request.multipart requires files_field and to_body_field")
		}
	}
	if s.Response.Usage.Unit != "" {
		switch s.Response.Usage.Unit {
		case UnitImage, UnitSecond, UnitToken, UnitAction:
		default:
			return fmt.Errorf("response.usage.unit must be one of image|second|token|action, got %q", s.Response.Usage.Unit)
		}
	}
	return nil
}

// IsTerminalSuccess 判定上游状态串是否为成功终态。
func (s *Schema) IsTerminalSuccess(status string) bool {
	return containsFold(s.TaskStates.Success, status)
}

// IsTerminalFailure 判定上游状态串是否为失败终态。
func (s *Schema) IsTerminalFailure(status string) bool {
	return containsFold(s.TaskStates.Failure, status)
}

func containsFold(list []string, v string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}
