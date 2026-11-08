package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// MediaHubController serves configuration even when generation has not been enabled yet.
type MediaHubController struct {
	settings *mediahub.SettingsStore
	bindings *mediahub.BindingStore
	relay    *MediaHubRelayHandler
	links    *mediahub.ResultLinkStore
}

func DefaultMediaHubSettings() mediahub.Settings {
	c := LoadMediaHubConfig()
	return mediahub.Settings{Enabled: c.MediadBaseURL != "" && c.RelaySecret != "" && c.CallbackSecret != "", MediadBaseURL: c.MediadBaseURL, ManagementToken: os.Getenv("MEDIAHUB_MANAGEMENT_TOKEN"), RelaySecret: c.RelaySecret, CallbackSecret: c.CallbackSecret, StorePath: c.StorePath, ForwardSeconds: int(c.ForwardTimeout / time.Second), ManagementSeconds: 30}
}

func NewMediaHubController(path string, defaults mediahub.Settings, oai *OpenAIGatewayHandler, links ...*mediahub.ResultLinkStore) (*MediaHubController, error) {
	store, err := mediahub.OpenSettings(path, defaults)
	if err != nil {
		return nil, err
	}
	s := store.Active()
	bindings, err := mediahub.NewBindingStore(s.StorePath)
	if err != nil {
		return nil, err
	}
	h := &MediaHubController{settings: store, bindings: bindings}
	if len(links) > 0 {
		h.links = links[0]
	}
	if s.Enabled && oai != nil {
		h.relay, err = newMediaHubRelay(oai, MediaHubConfig{MediadBaseURL: s.MediadBaseURL, RelaySecret: s.RelaySecret, CallbackSecret: s.CallbackSecret, StorePath: s.StorePath, ForwardTimeout: time.Duration(s.ForwardSeconds) * time.Second})
		if err != nil {
			return nil, err
		}
		h.relay.bindings = bindings
		if len(links) > 0 {
			h.relay.links = links[0]
		}
	}
	return h, nil
}

func InitMediaHub(h *Handlers) error {
	if h.MediaHub != nil {
		return nil
	}
	path := os.Getenv("MEDIAHUB_CONFIG_PATH")
	if path == "" {
		path = filepath.Join("data", "mediahub_settings.json")
	}
	var err error
	h.MediaHub, err = NewMediaHubController(path, DefaultMediaHubSettings(), h.OpenAIGateway, h.MediaLinks)
	if err == nil && h.MediaHub.relay != nil && h.Gateway != nil {
		h.MediaHub.relay.accounts = h.Gateway.gatewayService
	}
	return err
}

func (h *MediaHubController) Relay(c *gin.Context) {
	if h.relay == nil {
		response.Error(c, 503, "图片与视频服务未启用，保存配置后请重启主站")
		return
	}
	h.relay.Relay(c)
}
func (h *MediaHubController) Callback(c *gin.Context) {
	if h.relay == nil {
		response.Error(c, 503, "图片与视频服务未启用")
		return
	}
	h.relay.SettleCallback(c)
}

// RegisterAdmin must only be called on an authenticated administrator group.
func (h *MediaHubController) RegisterAdmin(admin *gin.RouterGroup) {
	g := admin.Group("/media-service")
	g.GET("/records", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
		if err != nil || id <= 0 {
			response.Error(c, 400, "请输入有效用户 ID")
			return
		}
		records, err := h.links.List(c.Request.Context(), id, "")
		if err != nil {
			response.Error(c, 503, "下载记录数据库不可用")
			return
		}
		response.Success(c, gin.H{"records": records})
	})
	g.GET("/config", h.getConfig)
	g.PUT("/config", h.putConfig)
	g.GET("/bindings", func(c *gin.Context) { response.Success(c, gin.H{"bindings": h.bindings.List()}) })
	g.PUT("/bindings/:id", h.putBinding)
	g.DELETE("/bindings/:id", h.deleteBinding)
	g.Any("/agent/*path", h.agent)
}

func (h *MediaHubController) getConfig(c *gin.Context) { response.Success(c, h.settings.View()) }
func (h *MediaHubController) putConfig(c *gin.Context) {
	next := h.settings.Desired()
	if err := c.ShouldBindJSON(&next); err != nil {
		response.Error(c, 400, "配置格式不正确")
		return
	}
	if err := h.settings.Update(next); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, h.settings.View())
}

var mediaID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func (h *MediaHubController) putBinding(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, 400, "分组 ID 必须是正整数")
		return
	}
	var b mediahub.Binding
	if err = c.ShouldBindJSON(&b); err != nil {
		response.Error(c, 400, "绑定格式不正确")
		return
	}
	b.GroupID = id
	if !mediaID.MatchString(b.SchemaID) || (b.VideoSchemaID != "" && !mediaID.MatchString(b.VideoSchemaID)) || (b.EstimateKind != "image" && b.EstimateKind != "video" && b.EstimateKind != "token") {
		response.Error(c, 400, "请选择有效规则和图片、视频或 Token 估算方式")
		return
	}
	if b.Enabled {
		for _, schema := range []string{b.SchemaID, b.VideoSchemaID} {
			if schema == "" {
				continue
			}
			if _, err = h.client().Request(c.Request.Context(), "GET", "/management/schemas/"+url.PathEscape(schema), nil); err != nil {
				response.Error(c, 400, "启用前请先保存对应规则并确认微服务可用")
				return
			}
		}
	}
	if err = h.bindings.Put(b); err != nil {
		response.Error(c, 503, "绑定保存失败")
		return
	}
	response.Success(c, b)
}
func (h *MediaHubController) deleteBinding(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, 400, "分组 ID 不正确")
		return
	}
	deleted, err := h.bindings.Delete(id)
	if err != nil {
		response.Error(c, 503, "绑定删除失败")
		return
	}
	response.Success(c, gin.H{"deleted": deleted})
}
func (h *MediaHubController) client() *mediahub.ManagementClient {
	s := h.settings.Desired()
	return &mediahub.ManagementClient{BaseURL: s.MediadBaseURL, Token: s.ManagementToken, HTTP: &http.Client{Timeout: time.Duration(s.ManagementSeconds) * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func allowedMediaAgent(method, path string) bool {
	switch path {
	case "/capabilities":
		return method == "GET"
	case "/config", "/callback":
		return method == "GET" || method == "PUT"
	case "/schemas", "/tasks/pending":
		return method == "GET"
	case "/callback/test", "/dry-run":
		return method == "POST"
	}
	p := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(p) == 2 && p[0] == "schemas" && mediaID.MatchString(p[1]) {
		return method == "GET" || method == "PUT" || method == "DELETE"
	}
	return len(p) == 3 && p[0] == "tasks" && mediaID.MatchString(p[1]) && p[2] == "recover" && method == "POST"
}
func (h *MediaHubController) agent(c *gin.Context) {
	path, method := c.Param("path"), c.Request.Method
	if !allowedMediaAgent(method, path) {
		response.Error(c, 404, "接口不存在")
		return
	}
	if method == "DELETE" && strings.HasPrefix(path, "/schemas/") {
		for _, b := range h.bindings.List() {
			if b.Enabled && (b.SchemaID == strings.TrimPrefix(path, "/schemas/") || b.VideoSchemaID == strings.TrimPrefix(path, "/schemas/")) {
				response.Error(c, 409, "请先停用引用该规则的分组绑定")
				return
			}
		}
	}
	var body any
	if method == "PUT" || method == "POST" {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
		var raw json.RawMessage
		if err := c.ShouldBindJSON(&raw); err != nil {
			response.Error(c, 400, "请输入有效 JSON")
			return
		}
		body = raw
		if path == "/callback" {
			var cb mediahub.CallbackConfig
			if json.Unmarshal(raw, &cb) != nil {
				response.Error(c, 400, "通知配置不正确")
				return
			}
			// The main-site key is authoritative, and is never returned to the browser.
			if cb.CallbackSecret != "" {
				response.Error(c, 400, "请在主站连接配置中设置通知密钥")
				return
			}
			cb.CallbackSecret = h.settings.Desired().CallbackSecret
			body = cb
		}
	}
	endpoint := "/management" + path
	if path == "/capabilities" {
		endpoint = path
	}
	out, err := h.client().Request(c.Request.Context(), method, endpoint, body)
	if err != nil {
		response.Error(c, 502, fmt.Sprintf("微服务请求失败：%s", safeMediaError(err, h.settings.Desired())))
		return
	}
	response.Success(c, json.RawMessage(out))
}

func safeMediaError(err error, s mediahub.Settings) string {
	v := err.Error()
	for _, key := range []string{s.ManagementToken, s.RelaySecret, s.CallbackSecret} {
		if key != "" {
			v = strings.ReplaceAll(v, key, "[已隐藏]")
		}
	}
	return v
}
