package middleware

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func syncIsolationEndpoint(method, path string) (stream, count bool) {
	if method == http.MethodGet {
		count := strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/realtime")
		return count, count
	}
	if method != http.MethodPost {
		return false, false
	}
	if strings.HasSuffix(path, ":streamGenerateContent") {
		return true, true
	}
	if strings.HasSuffix(path, ":generateContent") {
		return false, true
	}
	for _, suffix := range []string{"/messages", "/chat/completions", "/responses", "/systemone", "/embeddings", "/images/generations", "/images/edits", "/tts", "/stt", "/alpha/search", "/web_search", "/x_search"} {
		if strings.HasSuffix(path, suffix) {
			return false, true
		}
	}
	return false, false
}

func syncIsolationStream(body []byte, contentType string, fallback bool) (bool, bool) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType == "multipart/form-data" {
		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		stream := fallback
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				return stream, true
			}
			if err != nil {
				return false, false
			}
			if part.FormName() == "stream" && part.FileName() == "" {
				value, err := io.ReadAll(io.LimitReader(part, 6))
				if err != nil || (string(value) != "true" && string(value) != "false") {
					return false, false
				}
				stream = string(value) == "true"
			}
			_ = part.Close()
		}
	}
	if !gjson.ValidBytes(body) {
		return false, false
	}
	value := gjson.GetBytes(body, "stream")
	if !value.Exists() {
		return fallback, true
	}
	if value.Type != gjson.True && value.Type != gjson.False {
		return false, false
	}
	return value.Bool(), true
}

func observeSyncIsolationRequest(c *gin.Context, svc *service.APIKeyService, key *service.APIKey, cfg *config.Config) {
	if key.Group == nil || (!key.Group.SyncIsolationEnabled && key.Group.SyncIsolationSourceGroupID == 0) {
		return
	}
	stream, count := syncIsolationEndpoint(c.Request.Method, c.Request.URL.Path)
	if !count {
		return
	}
	if c.Request.Method == http.MethodPost {
		readBody := func() ([]byte, error) {
			mediaType, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if strings.HasPrefix(mediaType, "multipart/") {
				return httputil.ReadRequestBodyWithPrealloc(c.Request)
			}
			var maxBytes int64
			if cfg != nil {
				maxBytes = cfg.Gateway.MaxBodySize
			}
			return httputil.ReadLenientJSONRequestBodyWithPrealloc(c.Request, maxBytes)
		}
		body, err := readBody()
		if err != nil {
			if !RejectRequestBodyReadError(c, "middleware.sync_isolation", err) {
				AbortWithRequestBodyReadFailure(c, "middleware.sync_isolation", err)
			}
			return
		}
		requestmodel.ResetRequestBody(c.Request, body)
		var valid bool
		stream, valid = syncIsolationStream(body, c.GetHeader("Content-Type"), stream)
		if !valid {
			return
		}
		stream = syncIsolationEffectiveStream(c.Request.URL.Path, stream)
	}
	svc.QueueSyncIsolation(key, stream)
}

func syncIsolationEffectiveStream(path string, requested bool) bool {
	if strings.HasSuffix(path, ":streamGenerateContent") {
		return true
	}
	for _, suffix := range []string{":generateContent", "/systemone", "/embeddings", "/stt", "/alpha/search", "/web_search", "/x_search"} {
		if strings.HasSuffix(path, suffix) {
			return false
		}
	}
	return requested
}
