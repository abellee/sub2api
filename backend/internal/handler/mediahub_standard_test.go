package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	for _, protocol := range []string{"openai", "openai_async", "gemini", "grok_image"} {
		t.Run(protocol, func(t *testing.T) {
			stateQueries := 0
			media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get(mediahub.HeaderUserID) != "42" || r.Header.Get(mediahub.HeaderKeyID) != "7" {
					t.Error("missing signed identity")
				}
				if r.Method == "POST" {
					want := "gpt-image-2.6"
					if protocol == "gemini" {
						want = "gemini-3-pro-image"
					}
					if protocol == "grok_image" {
						want = "grok-imagine-image-quality"
					}
					if r.Header.Get(mediahub.HeaderModel) != want {
						t.Error("model missing from relay", r.Header.Get(mediahub.HeaderModel))
					}
					w.WriteHeader(202)
					_, _ = w.Write([]byte(`{"task_id":"mtask_standard"}`))
					return
				}
				switch r.URL.Query().Get("response_format") {
				case "state":
					stateQueries++
					status, callback := "processing", "pending"
					if stateQueries > 1 || protocol == "openai_async" {
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
			if protocol == "gemini" {
				path = "/v1beta/models/gemini-3-pro-image:generateContent"
				body = `{"contents":[{"parts":[{"text":"test"}]}]}`
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
			if protocol == "openai_async" {
				if w.Code != 202 || gjson.Get(w.Body.String(), "poll_url").String() != "/v1/images/tasks/mtask_standard" {
					t.Fatal(w.Code, w.Body.String())
				}
				w = httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", "/v1/images/tasks/mtask_standard", nil))
				if w.Code != 200 || gjson.Get(w.Body.String(), "status").String() != "completed" {
					t.Fatal(w.Code, w.Body.String())
				}
			} else if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			} else if protocol == "gemini" && !gjson.Get(w.Body.String(), "candidates.0.content.parts.0.inlineData.data").Exists() {
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
	out, err := mediaImageMultipart(body.Bytes(), w.FormDataContentType())
	if err != nil {
		t.Fatal(err)
	}
	if !gjson.GetBytes(out, "stream").Bool() || gjson.GetBytes(out, "n").Int() != 2 || len(gjson.GetBytes(out, "images").Array()) != 2 || !strings.HasPrefix(gjson.GetBytes(out, "mask.image_url").String(), "data:image/png;base64,") {
		t.Fatal("multipart options or files lost")
	}
	if _, err = mediaImageMultipart(body.Bytes(), "multipart/form-data"); err == nil {
		t.Fatal("missing boundary accepted")
	}
}

func TestMediaImageStreamFlushesBeforeCompletion(t *testing.T) {
	for _, endpoint := range []string{"generations", "edits"} {
		t.Run(endpoint, func(t *testing.T) {
			release := make(chan struct{})
			media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("response_format") == "state" {
					<-release
					_, _ = w.Write([]byte(`{"status":"succeeded","callback_state":"confirmed"}`))
					return
				}
				if r.URL.Query().Get("response_format") != "openai_b64_json" {
					t.Error("stream must contain base64")
				}
				_, _ = w.Write([]byte(`{"created":123,"output_format":"png","data":[{"b64_json":"aW1hZ2Ux","download_url":"https://d.tokenfree.dev/1"},{"b64_json":"aW1hZ2Uy","download_url":"https://d.tokenfree.dev/2"}]}`))
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
			defer resp.Body.Close()
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
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Header("Content-Type", "text/event-stream")
	c.Writer.Flush()
	mediaStreamResult(c, &mediahub.ForwardResponse{StatusCode: 400, Body: []byte(`{"error":{"code":"content_policy_violation","message":"blocked"}}`)})
	if !strings.Contains(w.Body.String(), "event: error") || !strings.Contains(w.Body.String(), "content_policy_violation") {
		t.Fatal(w.Body.String())
	}
}
