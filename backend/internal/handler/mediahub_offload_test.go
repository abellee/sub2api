package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Run with MEDIA_OFFLOAD_AGENT pointing at the mediad binary built from this checkout.
func TestMediaOffloadWithRealAgent(t *testing.T) {
	binary := os.Getenv("MEDIA_OFFLOAD_AGENT")
	if binary == "" {
		t.Skip("MEDIA_OFFLOAD_AGENT is not set")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	mediaURL := "http://" + addr
	dir := t.TempDir()
	var encoded bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	var paidCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paidCalls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/images/generations" || gjson.GetBytes(raw, "model").String() != "gpt-image-2.6" || r.Header.Get("Authorization") != "Bearer fixture-upstream" {
			t.Error("media did not assemble upstream request")
		}
		<-release
		item := map[string]any{"url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{item, item}})
	}))
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		upstream.Close()
	}()
	links, _ := mediaLinkFixture(t)
	bindings, err := mediahub.NewBindingStore(filepath.Join(dir, "bindings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := bindings.Put(mediahub.Binding{GroupID: 3, SchemaID: "fixture", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	var bills atomic.Int32
	h := &MediaHubRelayHandler{links: links, bindings: bindings, cfg: MediaHubConfig{MediadBaseURL: mediaURL, RelaySecret: "fixture-relay", ForwardTimeout: 5 * time.Second}, client: &http.Client{}, upstreamResolver: func(int64, string) (string, string) { return upstream.URL, "fixture-upstream" }, billingSink: mediaResultSinkFunc(func(mediahub.SettleRequest) error { bills.Add(1); return nil })}
	h.settle = mediahub.NewSettleServer(func() string { return "fixture-callback" }, &mediaHubSettleChain{h: h}, time.Hour)
	controller := &MediaHubController{bindings: bindings, relay: h}
	router := gin.New()
	var mainRequests atomic.Int32
	router.POST("/callback", h.SettleCallback)
	router.POST("/v1/images/generations", func(c *gin.Context) {
		mainRequests.Add(1)
		gid := int64(3)
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid})
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		controller.TryStandardImage(c, false)
	})
	main := httptest.NewServer(router)
	defer main.Close()
	schema := `{"fixture":{"id":"fixture","version":1,"mode":"sync","upstream":{"create":{"method":"POST","path":"/v1/images/generations"}},"request":{"body_mode":"passthrough","credential":{"Authorization":"Bearer {credential}"}},"response":{"usage":{"quantity":"data","unit":"image"},"result":{"files":"data"}}}}`
	if err := os.WriteFile(filepath.Join(dir, "schemas.json"), []byte(schema), 0600); err != nil {
		t.Fatal(err)
	}
	callback, _ := json.Marshal(map[string]any{"callback_url": main.URL + "/callback", "callback_secret": "fixture-callback", "enabled": true})
	if err := os.WriteFile(filepath.Join(dir, "callback.json"), callback, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "MEDIAD_LISTEN="+addr, "MEDIAD_DATA_DIR="+dir, "MEDIAD_RESULT_DIR="+filepath.Join(dir, "media"), "MEDIAD_DOWNLOAD_BASE_URL="+mediaURL, "MEDIAD_RELAY_SECRET=fixture-relay", "MEDIAD_MANAGEMENT_TOKEN=fixture-admin", "MEDIAD_CONFIG_PATH="+filepath.Join(dir, "settings.json"))
	logFile, err := os.Create(filepath.Join(dir, "agent.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = logFile.Close() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(mediaURL + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			logs, _ := os.ReadFile(logFile.Name())
			t.Fatal("media startup failed", string(logs))
		}
		time.Sleep(10 * time.Millisecond)
	}
	response, err := http.Post(main.URL+"/v1/images/generations?async=true", "application/json", strings.NewReader(`{"model":"gpt-image-2.6","prompt":"fixture","n":2}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != 202 {
		t.Fatal(response.StatusCode, string(raw), err)
	}
	pollURL := gjson.GetBytes(raw, "poll_url").String()
	if !strings.HasPrefix(pollURL, mediaURL+"/downloads/") {
		t.Fatal("main did not return media link", string(raw))
	}
	close(release)
	for {
		resp, err := http.Get(pollURL)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Fatal("browser cannot query media", resp.StatusCode, string(raw))
		}
		if gjson.GetBytes(raw, "status").String() == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task did not settle", string(raw))
		}
		time.Sleep(20 * time.Millisecond)
	}
	items := gjson.GetBytes(raw, "result.data").Array()
	if len(items) != 2 {
		t.Fatal("multi-image result lost", string(raw))
	}
	for _, item := range items {
		resp, err := http.Get(item.Get("url").String())
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Fatal("canvas cannot download image", resp.StatusCode)
		}
		decoded, err := png.Decode(resp.Body)
		_ = resp.Body.Close()
		if err != nil || decoded.Bounds().Dx() != 2 {
			t.Fatal("result cannot be displayed", err)
		}
	}
	// Unmodified clients submit synchronously; only clients explicitly asking for Base64 receive it.
	for _, format := range []string{"", "b64_json"} {
		body := `{"model":"gpt-image-2.6","prompt":"fixture","n":2}`
		if format != "" {
			body = `{"model":"gpt-image-2.6","prompt":"fixture","n":2,"response_format":"b64_json"}`
		}
		resp, err := http.Post(main.URL+"/v1/images/generations", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || len(gjson.GetBytes(raw, "data").Array()) != 2 {
			t.Fatal(resp.StatusCode, string(raw), err)
		}
		for _, item := range gjson.GetBytes(raw, "data").Array() {
			if format == "" {
				if !strings.HasPrefix(item.Get("url").String(), mediaURL+"/downloads/") || item.Get("b64_json").Exists() {
					t.Fatal("default result must contain media links", string(raw))
				}
			} else {
				data, err := base64.StdEncoding.DecodeString(item.Get("b64_json").String())
				if err != nil || !bytes.Equal(data, encoded.Bytes()) {
					t.Fatal("explicit Base64 compatibility lost", err)
				}
			}
		}
	}
	if mainRequests.Load() != 3 || paidCalls.Load() != 3 || bills.Load() != 3 {
		t.Fatal("unexpected repeated gateway/paid/billing call", mainRequests.Load(), paidCalls.Load(), bills.Load())
	}
}
