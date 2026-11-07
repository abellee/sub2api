package handler

// MediaHubRelayHandler 把 sub2api 核心计费桥接到 mediahub SDK:
// - 冻结:复用 reserveInflightBalance(与 Grok 媒体同款在途预留)
// - 账号:复用 OpenAIGatewayService.SelectAccountForModel(分组内调度/故障转移)
// - 结算:复用 OpenAIGatewayService.RecordUsage(余额/配额/用量行全链路)
// 本文件是与核心逻辑的唯一衔接点;SDK 本体在 internal/mediahub(零核心依赖)。

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// MediaHubConfig 是 mediahub 的运行配置;从环境变量读取,
// 不改动 config.Config 结构,降低合并官方新版本时的冲突面。
type MediaHubConfig struct {
	// MediadBaseURL 是 mediad 服务地址(MEDIAHUB_MEDIAD_BASE_URL)。
	MediadBaseURL string
	// RelaySecret 是与 mediad 共享的中转签名密钥(MEDIAHUB_RELAY_SECRET)。
	RelaySecret string
	// CallbackSecret 是注册到 mediad 的回调签名密钥(MEDIAHUB_CALLBACK_SECRET)。
	CallbackSecret string
	// StorePath 是分组绑定存储文件(MEDIAHUB_STORE_PATH,默认 ./data/mediahub_bindings.json)。
	StorePath string
	// ForwardTimeout 是同步等待 mediad 的上限(MEDIAHUB_FORWARD_TIMEOUT_SECONDS,默认 900)。
	ForwardTimeout time.Duration
}

// LoadMediaHubConfig 从环境变量装配配置。
func LoadMediaHubConfig() MediaHubConfig {
	cfg := MediaHubConfig{
		MediadBaseURL:  os.Getenv("MEDIAHUB_MEDIAD_BASE_URL"),
		RelaySecret:    os.Getenv("MEDIAHUB_RELAY_SECRET"),
		CallbackSecret: os.Getenv("MEDIAHUB_CALLBACK_SECRET"),
		StorePath:      os.Getenv("MEDIAHUB_STORE_PATH"),
	}
	if cfg.StorePath == "" {
		cfg.StorePath = filepath.Join("data", "mediahub_bindings.json")
	}
	timeoutSec := 900
	if v := os.Getenv("MEDIAHUB_FORWARD_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}
	cfg.ForwardTimeout = time.Duration(timeoutSec) * time.Second
	return cfg
}

// MediaHubRelayHandler 是媒体中转的 gin 处理器。
type MediaHubRelayHandler struct {
	oai      *OpenAIGatewayHandler
	cfg      MediaHubConfig
	bindings *mediahub.BindingStore
	settle   *mediahub.SettleServer
	links    *mediahub.ResultLinkStore
	client   *http.Client
	// upstreamResolver 解析分组+模型对应的上游地址与凭证;
	// 生产实现走调度器(见 resolveUpstream),测试可注入。
	upstreamResolver func(groupID int64, model string) (string, string)
	// billingSink 是结算落点;生产为 mediaHubBillingSink,测试可替换。
	billingSink mediahub.SettlementSink
}

// mediaHubSettleChain records links before billing, then exposes them only after billing succeeds.
type mediaHubSettleChain struct{ h *MediaHubRelayHandler }

func (c *mediaHubSettleChain) Settle(req mediahub.SettleRequest) error {
	ctx := req.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Status == "succeeded" && len(req.Files) > 0 {
		if err := c.h.links.Prepare(ctx, req, c.h.cfg.MediadBaseURL); err != nil {
			return err
		}
		if err := c.h.billingSink.Settle(req); err != nil {
			return err
		}
		return c.h.links.MarkReady(ctx, req.UserID, req.TaskID)
	}
	return c.h.billingSink.Settle(req)
}

// NewMediaHubRelay 构建媒体中转处理器;oai 提供计费/调度/冻结等核心能力。
// mediad 未配置(MEDIAHUB_MEDIAD_BASE_URL 为空)时返回错误,路由注册方据此跳过。
func NewMediaHubRelay(oai *OpenAIGatewayHandler) (*MediaHubRelayHandler, error) {
	return newMediaHubRelay(oai, LoadMediaHubConfig())
}

func newMediaHubRelay(oai *OpenAIGatewayHandler, cfg MediaHubConfig) (*MediaHubRelayHandler, error) {
	if oai == nil {
		return nil, errors.New("mediahub: openai gateway handler is nil")
	}
	if cfg.MediadBaseURL == "" {
		return nil, errors.New("mediahub: MEDIAHUB_MEDIAD_BASE_URL is not configured")
	}
	bindings, err := mediahub.NewBindingStore(cfg.StorePath)
	if err != nil {
		return nil, fmt.Errorf("mediahub: load bindings: %w", err)
	}
	h := &MediaHubRelayHandler{
		oai:      oai,
		cfg:      cfg,
		bindings: bindings,
		client:   &http.Client{Timeout: 0}, // 超时由请求 context 控制
	}
	h.upstreamResolver = h.resolveUpstream
	h.billingSink = &mediaHubBillingSink{h: h}
	h.settle = mediahub.NewSettleServer(
		func() string { return cfg.CallbackSecret },
		&mediaHubSettleChain{h: h},
		24*time.Hour,
	)
	return h, nil
}

// Bindings 暴露绑定存储(管理接口用)。
func (h *MediaHubRelayHandler) Bindings() *mediahub.BindingStore { return h.bindings }

// Config 返回运行配置(管理接口用;不含密钥明文)。
func (h *MediaHubRelayHandler) Config() MediaHubConfig { return h.cfg }

// Relay 是客户端媒体请求入口:ANY /media/*path(apiKey 鉴权后到达)。
// /media/files/* 返回用户的下载记录或跳转到中转服务，其余转发 mediad。
func (h *MediaHubRelayHandler) Relay(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"type": "authentication_error", "message": "invalid api key"}})
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	path := c.Param("path")
	if strings.HasPrefix(path, "/files") {
		h.serveFiles(c, subject.UserID, path)
		return
	}
	gid := int64(0)
	if apiKey.GroupID != nil {
		gid = *apiKey.GroupID
	}
	// 任务状态轮询:GET /media/tasks/{id}(兼容 /media/relay/tasks/{id})→
	// 签名 GET 转发,任务归属由 mediad 按身份头强制校验。
	if trimmed := strings.TrimPrefix(path, "/"); strings.HasPrefix(trimmed, "tasks/") || strings.HasPrefix(trimmed, "relay/tasks/") {
		taskID := strings.TrimPrefix(strings.TrimPrefix(trimmed, "relay/"), "tasks/")
		h.taskStatus(c, subject.UserID, apiKey.ID, gid, taskID)
		return
	}
	binding, ok := h.bindings.Get(gid)
	if !ok || !binding.Enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "media relay is not enabled for this group"}})
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 128<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "read body failed"}})
		return
	}
	model := gjson.GetBytes(body, "model").String()

	// 冻结:媒体生成为完成后计费,按绑定声明的估算口径在途预留,
	// 防止并发击穿与弃单白嫖(查询/轮询不冻结,不拦截用户取回已付费结果)。
	// h.oai 为 nil 仅出现在测试夹具(生产由 NewMediaHubRelay 强制非 nil)。
	if h.oai != nil {
		subscription, _ := middleware2.GetSubscriptionFromContext(c)
		inflightDone, inflightErr := reserveInflightBalance(c, h.oai.billingCacheService, h.oai.gatewayService,
			apiKey, subscription, inflightEstimate(binding, model, body))
		if inflightErr != nil {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": gin.H{"type": "insufficient_balance", "message": "insufficient balance for media generation"}})
			return
		}
		defer inflightDone()
	}

	// 账号信息:mediahub 绑定分组,该分组下的 OpenAI 平台账号即 mediad 账号
	// (Base URL 指向 mediad / 或由绑定配置直接指定上游)。
	upstreamURL, credential := h.upstreamResolver(gid, model)
	if upstreamURL == "" || credential == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "no media account configured for this group"}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.cfg.ForwardTimeout)
	defer cancel()

	resp, err := mediahub.Forward(ctx, h.client, mediahub.ForwardInput{
		MediadBaseURL:  h.cfg.MediadBaseURL,
		RelaySecret:    h.cfg.RelaySecret,
		SchemaID:       binding.SchemaID,
		UpstreamURL:    upstreamURL,
		Credential:     credential,
		ClientPath:     c.Param("path"),
		Body:           body,
		ContentType:    c.GetHeader("Content-Type"),
		UpstreamModel:  model,
		UserID:         subject.UserID,
		KeyID:          apiKey.ID,
		GroupID:        gid,
		IdempotencyKey: c.GetHeader("X-Idempotency-Key"),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "api_error", "message": err.Error()}})
		return
	}
	if resp.StatusCode == http.StatusAccepted {
		var accepted map[string]any
		if json.Unmarshal(resp.Body, &accepted) == nil {
			if taskID, ok := accepted["task_id"].(string); ok && taskID != "" {
				accepted["poll_url"] = "/media/tasks/" + taskID
				resp.Body, _ = json.Marshal(accepted)
			}
		}
	}

	// 同步路径:mediad 在 2xx 响应头带 usage,直接结算。Ctx 携带请求上下文,
	// 使在途预留与扣费原子抵扣;结算经 worker pool 提交(强制不丢弃),不阻塞响应。
	if resp.Usage != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		req := mediahub.SettleRequest{
			RequestID: fmt.Sprintf("media_sync_%d_%d", apiKey.ID, time.Now().UnixNano()),
			APIKeyID:  apiKey.ID,
			UserID:    subject.UserID,
			GroupID:   gid,
			Model:     model,
			Usage:     resp.Usage,
			Status:    "succeeded",
			Ctx:       c.Request.Context(),
		}
		if err := h.billingSink.Settle(req); err != nil {
			// 构建结算任务失败(而非计费本身):记录以便对账兜底。
			logger.L().Error("mediahub sync settle failed", zap.Error(err))
		}
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, resp.Body)
}

// taskStatus 转发任务状态查询(GET,无 body;不冻结——取回已付费结果不设卡)。
func (h *MediaHubRelayHandler) taskStatus(c *gin.Context, userID, keyID, groupID int64, taskID string) {
	if taskID == "" || strings.ContainsAny(taskID, "/\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "invalid task id"}})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	resp, err := mediahub.ForwardTaskStatus(ctx, h.client, mediahub.ForwardInput{
		MediadBaseURL: h.cfg.MediadBaseURL,
		RelaySecret:   h.cfg.RelaySecret,
		UserID:        userID,
		KeyID:         keyID,
		GroupID:       groupID,
	}, taskID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "api_error", "message": err.Error()}})
		return
	}
	var state map[string]any
	if json.Unmarshal(resp.Body, &state) == nil {
		if files, ok := state["files"].([]any); ok && len(files) > 0 {
			delete(state, "files")
			delete(state, "result")
			links, linkErr := h.links.List(ctx, userID, taskID)
			if linkErr != nil {
				c.JSON(503, gin.H{"error": gin.H{"message": "media download records unavailable"}})
				return
			}
			if len(links) == len(files) {
				state["result"] = gin.H{"files": links}
			} else {
				state["status"] = "processing"
			}
		}
		resp.Body, _ = json.Marshal(state)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, resp.Body)
}

// inflightEstimate 按绑定声明的口径构造冻结估算:
// image(默认,按张)/ video(按秒,duration 取自请求体)/ token。
func inflightEstimate(binding mediahub.Binding, model string, body []byte) service.InflightEstimateRequest {
	switch strings.ToLower(strings.TrimSpace(binding.EstimateKind)) {
	case "video":
		duration := int(gjson.GetBytes(body, "duration").Int())
		if duration <= 0 {
			duration = 5
		}
		return service.InflightEstimateRequest{
			Model: model, Kind: service.InflightEstimateVideo,
			Units: 1, VideoDurationSeconds: duration,
		}
	case "token":
		return service.InflightEstimateRequest{
			Model: model, Kind: service.InflightEstimateToken,
			BodyBytes: len(body), MaxTokens: 1,
		}
	default:
		return service.InflightEstimateRequest{Model: model, Kind: service.InflightEstimateImage, Units: 1}
	}
}

// serveFiles returns only database metadata or redirects to the media-service download URL.
func (h *MediaHubRelayHandler) serveFiles(c *gin.Context, userID int64, path string) {
	if c.Request.Method != http.MethodGet {
		c.JSON(405, gin.H{"error": gin.H{"message": "GET required; files are stored by the media service"}})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	seg := strings.Split(strings.Trim(path, "/"), "/")
	if len(seg) == 0 || seg[0] != "files" {
		c.Status(404)
		return
	}
	if len(seg) == 1 {
		records, err := h.links.List(ctx, userID, "")
		if err != nil {
			c.JSON(503, gin.H{"error": gin.H{"message": "media download records unavailable"}})
			return
		}
		c.JSON(200, gin.H{"object": "list", "data": records})
		return
	}
	if len(seg) > 3 || (len(seg) == 3 && seg[2] != "content") {
		c.Status(404)
		return
	}
	record, err := h.links.Get(ctx, userID, seg[1])
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			c.JSON(503, gin.H{"error": gin.H{"message": "media download records unavailable"}})
			return
		}
		c.JSON(404, gin.H{"error": gin.H{"message": "file not found"}})
		return
	}
	if len(seg) == 3 {
		c.Header("Cache-Control", "private, no-store")
		c.Redirect(http.StatusTemporaryRedirect, record.DownloadURL)
		return
	}
	c.JSON(200, record)
}

// SettleCallback 是 mediad 终态回调入口:POST /internal/media/settle。
func (h *MediaHubRelayHandler) SettleCallback(c *gin.Context) {
	h.settle.ServeHTTP(c.Writer, c.Request)
}

// resolveUpstream 解析该分组媒体请求的上游地址与凭证。
// v1:经调度器选中分组内账号,取其平台 Base URL 与凭证;
// 账号指向 mediad 或直接指向上游均可(组装规则决定最终形态)。
func (h *MediaHubRelayHandler) resolveUpstream(groupID int64, model string) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gid := groupID
	account, err := h.oai.gatewayService.SelectAccountForModel(ctx, &gid, "", model)
	if err != nil || account == nil {
		logger.L().Warn("mediahub: select account failed",
			zap.Int64("group_id", groupID), zap.String("model", model), zap.Error(err))
		return "", ""
	}
	return account.GetBaseURL(), account.GetCredential("api_key")
}

// mediaHubBillingSink 把 SDK 的结算请求翻译成核心 RecordUsage 调用。
// 价格和扣费只在主站执行；second 复用已有的视频计费字段。
type mediaHubBillingSink struct {
	h *MediaHubRelayHandler
}

// Settle 实现 mediahub.SettlementSink。
func (s *mediaHubBillingSink) Settle(req mediahub.SettleRequest) error {
	if req.Status != "succeeded" {
		return nil
	}
	if req.Usage == nil || req.Usage.Quantity <= 0 {
		return errors.New("mediahub: successful result missing usage metadata for main-site billing")
	}
	result, err := mediaHubUsageResult(req)
	if err != nil {
		return err
	}
	// 计费上下文:同步路径携带请求 ctx(在途预留原子抵扣);回调路径用 Background。
	billingCtx := req.Ctx
	if billingCtx == nil {
		billingCtx = context.Background()
	}
	apiKey, err := s.h.oai.apiKeyService.GetByID(billingCtx, req.APIKeyID)
	if err != nil || apiKey == nil {
		return fmt.Errorf("mediahub: load api key %d: %w", req.APIKeyID, err)
	}
	if apiKey.User == nil {
		return fmt.Errorf("mediahub: api key %d has no user", req.APIKeyID)
	}
	if apiKey.User.ID != req.UserID {
		return errors.New("mediahub: task owner does not match API key")
	}
	var gid *int64
	if apiKey.GroupID != nil {
		gid = apiKey.GroupID
	}
	account, err := s.h.oai.gatewayService.SelectAccountForModel(billingCtx, gid, "", req.Model)
	if err != nil || account == nil {
		return fmt.Errorf("mediahub: select account for model %s: %w", req.Model, err)
	}

	// 回调确认必须等实际入账成功，否则微服务无法知道主站还需重试。
	return s.h.oai.gatewayService.RecordUsage(billingCtx, &service.OpenAIRecordUsageInput{
		Result:           result,
		APIKey:           apiKey,
		User:             apiKey.User,
		Account:          account,
		InboundEndpoint:  "/media",
		UpstreamEndpoint: "/media",
		APIKeyService:    s.h.oai.apiKeyService,
	})
}

func mediaHubUsageResult(req mediahub.SettleRequest) (*service.OpenAIForwardResult, error) {
	result := &service.OpenAIForwardResult{RequestID: req.RequestID, Model: req.Model, BillingModel: req.Model}
	if req.Usage == nil {
		return result, nil
	}
	q := req.Usage.Quantity
	if math.IsNaN(q) || math.IsInf(q, 0) || q <= 0 || q > float64(math.MaxInt32) {
		return nil, errors.New("mediahub: invalid usage quantity")
	}
	switch req.Usage.Unit {
	case mediahub.UnitImage:
		if q != math.Trunc(q) {
			return nil, errors.New("mediahub: image count must be an integer")
		}
		result.ImageEndpoint = true
		result.ImageCount = int(q)
		result.ImageSize = "1K"
		if size, ok := req.Parameters["size"].(string); ok && size != "" {
			result.ImageSize = size
		}
	case mediahub.UnitToken:
		result.Usage = service.OpenAIUsage{OutputTokens: int(math.Ceil(q))}
	case mediahub.UnitSecond:
		// 复用主站现有时长范围，超范围不静默按 15 秒少收。
		if q > service.VideoBillingMaxDurationSeconds {
			return nil, errors.New("mediahub: video duration exceeds current main-site billing range")
		}
		result.VideoCount = 1
		result.VideoDurationSeconds = int(math.Ceil(q))
		resolution, _ := req.Parameters["resolution"].(string)
		if value, ok := req.Result["resolution"].(string); ok && value != "" {
			resolution = value
		}
		result.VideoResolution = service.NormalizeVideoBillingResolutionOrDefault(resolution)
	default:
		return nil, fmt.Errorf("mediahub: unsupported billing unit %q", req.Usage.Unit)
	}
	return result, nil
}

// mediaHubAdminView 是管理接口的配置视图(脱敏)。
func (h *MediaHubRelayHandler) mediaHubAdminView() map[string]any {
	return map[string]any{
		"mediad_base_url":     h.cfg.MediadBaseURL,
		"store_path":          h.cfg.StorePath,
		"forward_timeout":     h.cfg.ForwardTimeout.String(),
		"relay_configured":    h.cfg.RelaySecret != "",
		"callback_configured": h.cfg.CallbackSecret != "",
	}
}
