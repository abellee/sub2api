// Package mediahub 是 sub2api 侧对接 mediad(组装中转媒体服务)的 SDK。
//
// 设计目标:与 sub2api 核心逻辑隔离——本包不 import 任何核心 service/handler,
// 只依赖标准库与 gin 适配所需的类型;与核心的衔接通过 SettlementSink 接口与
// handler 包内的薄桥(mediahub_handler.go)完成,保证合并官方新版本时冲突面最小。
//
// 契约:与 mediad 的接口规范见 mediad/README.md(feature/media-relay-service 分支);
// 签名算法与受信请求头在本包 signature.go 中镜像实现。
package mediahub

import "context"

// 契约头名称,与 mediad/internal/relay/handler.go 保持一致。
const (
	HeaderTimestamp      = "X-Media-Timestamp"
	HeaderSignature      = "X-Media-Signature"
	HeaderCredential     = "X-Media-Credential"
	HeaderUpstreamURL    = "X-Media-Upstream-URL"
	HeaderSchemaVersion  = "X-Media-Schema-Version"
	HeaderIdempotencyKey = "X-Media-Idempotency-Key"
	HeaderModel          = "X-Media-Model"
	HeaderUserID         = "X-Sub2API-User-Id"
	HeaderKeyID          = "X-Sub2API-Key-Id"
	HeaderGroupID        = "X-Sub2API-Group-Id"
	HeaderUsage          = "X-Media-Usage"
	HeaderUsageError     = "X-Media-Usage-Error"
	HeaderEvent          = "X-Media-Event"
	// HeaderResult 是 mediad 在同步响应中携带结果提取(可选,JSON):
	// 文件落盘的下载来源优先取它,缺失时回退扫描响应体。
	HeaderResult = "X-Media-Result"
)

// 计费单位(与 mediad 契约一致)。
const (
	UnitImage  = "image"
	UnitSecond = "second"
	UnitToken  = "token"
	UnitAction = "action"
)

// Usage 是一次成功生成的计费用量;价格换算在 sub2api 侧。
type Usage struct {
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

// ResultFile 是微服务保存的文件说明，主站只记录下载链接和归属。
type ResultFile struct {
	DownloadURL string `json:"download_url"`
	ID          string `json:"file_id"`
	Path        string `json:"path"`
	Size        int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
}

// SettleRequest 是结算请求:由同步 usage 头或异步回调触发。
type SettleRequest struct {
	// TaskID 是 mediad 内部任务号(异步);同步请求为空。
	TaskID string
	// RequestID 用作幂等键(usage 记录 request_id),形如 media_task_settle:{task_id}。
	RequestID string
	// APIKeyID/UserID/GroupID 标识归属。
	APIKeyID int64
	UserID   int64
	GroupID  int64
	// Model 是改写后的上游模型名(计费按此查价)。
	Model string
	// Usage 为 nil 表示失败/不计费(仅用于触发冻结释放)。
	Usage *Usage
	// Status 是任务终态(succeeded|failed);同步结算为 "succeeded"。
	Status string
	// Result 是 mediad 回调带回的结果提取(如 image_urls),
	// 文件字节始终保存在微服务，主站不下载副本。
	Result     map[string]any
	Files      []ResultFile
	Parameters map[string]any
	// Ctx 是计费上下文(同步路径携带请求 context,便于在途预留与扣费
	// 原子抵扣);回调路径为 nil,结算实现应回退 context.Background()。
	Ctx context.Context
}
