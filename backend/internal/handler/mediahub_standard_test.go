package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func TestMediaStandardEndpoints(t *testing.T) {
	for _, protocol := range []string{"openai", "openai_async", "gemini_async", "gemini", "grok_image"} {
		t.Run(protocol, func(t *testing.T) {
			stateQueries := 0
			media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get(mediahub.HeaderUserID) != "42" || r.Header.Get(mediahub.HeaderKeyID) != "7" {
					t.Error("missing signed identity")
				}
				if r.Method == "POST" {
					want := "gpt-image-2.6"
					if strings.HasPrefix(protocol, "gemini") {
						want = "gemini-3-pro-image"
					}
					if protocol == "grok_image" {
						want = "grok-imagine-image-quality"
					}
					if r.Header.Get(mediahub.HeaderModel) != want {
						t.Error("model missing from relay", r.Header.Get(mediahub.HeaderModel))
					}
					w.WriteHeader(202)
					_, _ = w.Write([]byte(`{"task_id":"mtask_standard","result_url":"https://media.example/downloads/mtask_standard/result?token=opaque"}`))
					return
				}
				switch r.URL.Query().Get("response_format") {
				case "state":
					stateQueries++
					status, callback := "processing", "pending"
					if stateQueries > 1 || strings.HasSuffix(protocol, "_async") {
						status = "succeeded"
						callback = "confirmed"
					}
					_ = json.NewEncoder(w).Encode(gin.H{"status": status, "callback_state": callback})
				case "gemini":
					_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}}],"download_urls":["https://d.tokenfree.dev/downloads/test"]}`))
				case "openai_url":
					_, _ = w.Write([]byte(`{"created":123,"data":[{"url":"https://d.tokenfree.dev/downloads/test"}]}`))
				default:
					t.Error("unexpected result format", r.URL.RawQuery)
				}
			}))
			defer media.Close()
			bindings, err := mediahub.NewBindingStore(filepath.Join(t.TempDir(), "bindings.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err = bindings.Put(mediahub.Binding{GroupID: 3, SchemaID: "test", Enabled: true}); err != nil {
				t.Fatal(err)
			}
			relay := &MediaHubRelayHandler{bindings: bindings, cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test", ForwardTimeout: 2 * time.Second}, client: media.Client(), upstreamResolver: func(int64, string) (string, string) { return "https://upstream.example", "test" }}
			controller := &MediaHubController{bindings: bindings, relay: relay}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				gid := int64(3)
				group := &service.Group{}
				if protocol == "grok_image" {
					group.Platform = service.PlatformGrok
				}
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid, Group: group})
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			})
			router.POST("/v1/images/generations", func(c *gin.Context) { controller.TryStandardImage(c, false) })
			router.POST("/v1/images/generations/async", func(c *gin.Context) { controller.TryStandardImage(c, true) })
			router.GET("/v1/images/tasks/:task_id", func(c *gin.Context) { controller.TryStandardImageTask(c) })
			router.POST("/v1beta/models/*modelAction", func(c *gin.Context) { controller.TryStandardGemini(c) })
			path, body := "/v1/images/generations", `{"model":"gpt-image-2.6","prompt":"test","response_format":"url"}`
			if strings.HasPrefix(protocol, "gemini") {
				path = "/v1beta/models/gemini-3-pro-image:generateContent"
				body = `{"contents":[{"parts":[{"text":"test"}]}]}`
			}
			if protocol == "gemini_async" {
				path += "?async=true"
			}
			if protocol == "openai_async" {
				path += "/async"
			}
			if protocol == "grok_image" {
				body = `{"model":"grok-imagine","prompt":"test"}`
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", path, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, r)
			if strings.HasSuffix(protocol, "_async") {
				if w.Code != 202 || gjson.Get(w.Body.String(), "poll_url").String() != "/v1/images/tasks/mtask_standard" {
					t.Fatal(w.Code, w.Body.String())
				}
				w = httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", "/v1/images/tasks/mtask_standard", nil))
				if w.Code != 200 || gjson.Get(w.Body.String(), "status").String() != "completed" {
					t.Fatal(w.Code, w.Body.String())
				}
			} else if stateQueries != 0 {
				t.Fatal("main polled while waiting", stateQueries)
			} else if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			} else if strings.HasPrefix(protocol, "gemini") && !gjson.Get(w.Body.String(), "candidates.0.content.parts.0.inlineData.data").Exists() {
				t.Fatal("native response lost", w.Body.String())
			} else if protocol != "gemini" && !gjson.Get(w.Body.String(), "data.0.url").Exists() {
				t.Fatal("standard images response lost", w.Body.String())
			}
		})
	}
}

func TestMediaImageMultipart(t *testing.T) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{"model": "gpt-image-2.6", "stream": "true", "n": "2", "prompt": "keep this subject"} {
		_ = w.WriteField(k, v)
	}
	for _, field := range []string{"image[]", "image[]", "mask"} {
		part, err := w.CreateFormFile(field, "input.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write([]byte("\x89PNG\r\n\x1a\nimage"))
	}
	_ = w.Close()
	out, err := mediaImageMultipartMetadata(body.Bytes(), w.FormDataContentType())
	if err != nil {
		t.Fatal(err)
	}
	if !gjson.GetBytes(out, "stream").Bool() || gjson.GetBytes(out, "n").Int() != 2 || gjson.GetBytes(out, "images").Exists() || gjson.GetBytes(out, "mask").Exists() || gjson.GetBytes(out, "prompt").Exists() {
		t.Fatal("routing metadata lost or image bytes encoded by main")
	}
	if _, err = mediaImageMultipartMetadata(body.Bytes(), "multipart/form-data"); err == nil {
		t.Fatal("missing boundary accepted")
	}
}

func TestMediaImageStreamFlushesBeforeCompletion(t *testing.T) {
	for _, endpoint := range []string{"generations", "edits"} {
		t.Run(endpoint, func(t *testing.T) {
			release := make(chan struct{})
			media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantFormat := "openai_sse"
				event := "image_generation.completed"
				if endpoint == "edits" {
					wantFormat = "openai_edit_sse"
					event = "image_edit.completed"
				}
				if r.URL.Query().Get("response_format") != wantFormat || r.URL.Query().Get("wait") != "true" {
					t.Error("main must delegate one wait/stream to media", r.URL.RawQuery)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, ": keep-alive\n\n")
				flusher, ok := w.(http.Flusher)
				if !ok {
					t.Error("test server does not support flushing")
					return
				}
				flusher.Flush()
				<-release
				for _, image := range []string{"aW1hZ2Ux", "aW1hZ2Uy"} {
					_, _ = io.WriteString(w, "event: "+event+"\ndata: {\"b64_json\":\""+image+"\",\"created_at\":123}\n\n")
				}

			}))
			defer media.Close()
			h := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test", ForwardTimeout: 5 * time.Second}, client: media.Client()}
			router := gin.New()
			router.POST("/v1/images/"+endpoint, func(c *gin.Context) {
				c.Set(mediaProtocolKey, "openai")
				h.finishStandard(c, &mediahub.ForwardResponse{StatusCode: 202, Body: []byte(`{"task_id":"mtask_stream"}`)}, []byte(`{"stream":true,"response_format":"url"}`), 42, 7, 3)
			})
			server := httptest.NewServer(router)
			defer server.Close()
			resp, err := server.Client().Post(server.URL+"/v1/images/"+endpoint, "application/json", strings.NewReader(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			reader := bufio.NewReader(resp.Body)
			line, err := reader.ReadString('\n')
			if err != nil || line != ": keep-alive\n" {
				t.Fatal("heartbeat not flushed before completion", line, err)
			}
			close(release)
			rest, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			event := "image_generation.completed"
			if endpoint == "edits" {
				event = "image_edit.completed"
			}
			if strings.Count(string(rest), "event: "+event) != 2 || !strings.Contains(string(rest), `"b64_json":"aW1hZ2Uy"`) || !strings.Contains(string(rest), `"created_at":123`) {
				t.Fatal("invalid stream result", string(rest))
			}
		})
	}
}

func TestMediaStandardStreamFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: error\ndata: {\"error\":{\"code\":\"content_policy_violation\"}}\n\n")
	}))
	defer upstream.Close()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/images/generations", nil)
	c.Set(mediaProtocolKey, "openai")
	h := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: upstream.URL, ForwardTimeout: time.Second}, client: upstream.Client()}
	h.finishStandard(c, &mediahub.ForwardResponse{StatusCode: 202, Body: []byte(`{"task_id":"mtask_error"}`)}, []byte(`{"stream":true}`), 42, 7, 3)
	if !strings.Contains(w.Body.String(), "event: error") || !strings.Contains(w.Body.String(), "content_policy_violation") {
		t.Fatal(w.Body.String())
	}
}

type mediaResultErrorTransport struct{ err error }

func (tr mediaResultErrorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, tr.err
}

func TestMediaResultWaitFailureReturnsRecoveryTask(t *testing.T) {
	for _, protocol := range []string{"openai", "gemini", "grok_image"} {
		for _, cause := range []error{context.DeadlineExceeded, context.Canceled, errors.New("connection reset by peer")} {
			t.Run(protocol+"/"+cause.Error(), func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("POST", "/v1/images/generations", nil)
				c.Set(mediaProtocolKey, protocol)
				relay := &MediaHubRelayHandler{
					cfg:    MediaHubConfig{MediadBaseURL: "http://private-media.example", ForwardTimeout: time.Second},
					client: &http.Client{Transport: mediaResultErrorTransport{cause}},
				}
				relay.finishStandard(c, &mediahub.ForwardResponse{StatusCode: 202, Body: []byte(`{"task_id":"mtask_wait"}`)}, nil, 2, 7, 3)
				body := w.Body.String()
				if w.Code != 502 || w.Header().Get("X-Task-Id") != "mtask_wait" || gjson.Get(body, "task_id").String() != "mtask_wait" || gjson.Get(body, "poll_url").String() != "/v1/images/tasks/mtask_wait" {
					t.Fatalf("missing recovery information: %d %s", w.Code, body)
				}
				if gjson.Get(body, "error.message").String() != "media result unavailable; task continues in background" {
					t.Fatal("lost public error", body)
				}
				if protocol == "gemini" && (gjson.Get(body, "error.code").Int() != 502 || gjson.Get(body, "error.status").String() != "INTERNAL") {
					t.Fatal("lost Gemini error format", body)
				}
				if len(c.Errors) != 1 || !errors.Is(c.Errors[0].Err, cause) {
					t.Fatal("lost underlying connection error", c.Errors)
				}
				if strings.Contains(body, cause.Error()) || strings.Contains(body, "private-media.example") {
					t.Fatal("private connection detail leaked", body)
				}
			})
		}
	}
}
