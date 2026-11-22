package mediahub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStorageCfg(root string) StorageConfig {
	return StorageConfig{
		Enabled:             true,
		Root:                root,
		Retention:           time.Hour,
		MaxDownloadBytes:    1 << 20,
		MaxUploadBytes:      1 << 20,
		GCInterval:          time.Minute,
		DownloadConcurrency: 2,
	}
}

func pngBytes() []byte { return bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 300) }

func TestUserKeyImmutableAndNoNumericID(t *testing.T) {
	s := NewFileStorage(testStorageCfg(t.TempDir()), nil)

	k1, err := s.userKey(42)
	if err != nil {
		t.Fatalf("userKey: %v", err)
	}
	k2, err := s.userKey(42)
	if err != nil || k1 != k2 {
		t.Fatalf("key not immutable: %q vs %q (%v)", k1, k2, err)
	}
	if !reUserKey.MatchString(k1) {
		t.Fatalf("key format invalid: %q", k1)
	}
	kOther, _ := s.userKey(43)
	if kOther == k1 {
		t.Fatal("different users must get different keys")
	}
	// 数字 ID 不得出现在目录路径中:用户目录段必须整体匹配 key 格式。
	dir, key, _ := s.userDir(42)
	if filepath.Base(dir) != key || !reUserKey.MatchString(key) {
		t.Fatalf("user dir segment invalid: %q", filepath.Base(dir))
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("user dir not created: %v", err)
	}
}

func TestEnqueueFromPayloadURLDownload(t *testing.T) {
	root := t.TempDir()
	s := NewFileStorage(testStorageCfg(root), nil)

	png := pngBytes()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	t.Cleanup(upstream.Close)

	body := map[string]any{"data": []any{map[string]any{"url": upstream.URL + "/img.png"}}}
	buf, _ := json.Marshal(body)
	s.EnqueueFromPayload(buf, http.Header{}, 42, "mj-v8.2")

	waitFor(t, 5*time.Second, func() bool {
		files := s.List(42)
		return len(files) == 1 && files[0].Status == StatusCompleted
	})
	meta := s.List(42)[0]
	if meta.ContentType != "image/png" || meta.SizeBytes != int64(len(png)) {
		t.Fatalf("meta = %+v", meta)
	}
	if meta.SHA256 != sha256Hex(png) {
		t.Fatal("sha256 mismatch")
	}
	if !strings.HasSuffix(meta.Filename, ".png") {
		t.Fatalf("filename = %q", meta.Filename)
	}
	// 落盘路径不得含数字用户 ID。
	_, path, err := s.Get(42, meta.FileID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// 落盘路径的用户目录段必须是不可变 key,不得含数字用户 ID。
	if !reUserKey.MatchString(filepath.Base(filepath.Dir(path))) {
		t.Fatalf("path user segment invalid: %q", path)
	}
	if !strings.HasPrefix(filepath.Dir(path), root) {
		t.Fatalf("path outside root: %q", path)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, png) {
		t.Fatalf("content mismatch: %v", err)
	}
}

func TestEnqueueFromResultDataAndB64(t *testing.T) {
	s := NewFileStorage(testStorageCfg(t.TempDir()), nil)
	raw := pngBytes()
	// 一个 b64_json 字段 + 一个 data URL。
	result := map[string]any{
		"data": []any{
			map[string]any{"b64_json": base64Std(raw)},
			map[string]any{"image_url": "data:image/png;base64," + base64Std(append(append([]byte{}, raw...), '2'))},
		},
	}
	s.EnqueueFromResult(result, 7, "banana", "mtask_x")

	waitFor(t, 5*time.Second, func() bool {
		files := s.List(7)
		completed := 0
		for _, f := range files {
			if f.Status == StatusCompleted {
				completed++
			}
		}
		return completed == 2
	})
	for _, f := range s.List(7) {
		if f.TaskID != "mtask_x" || f.Model != "banana" {
			t.Fatalf("meta attribution = %+v", f)
		}
		_, path, err := s.Get(7, f.FileID)
		if err != nil {
			t.Fatalf("get %s: %v", f.FileID, err)
		}
		data, _ := os.ReadFile(path)
		// 两个文件内容不同(第二个带 '2' 后缀),但都必须以 raw 为前缀。
		if !bytes.HasPrefix(data, raw) {
			t.Fatalf("decoded content mismatch for %s", f.FileID)
		}
	}
}

func TestGetRejectsTraversalAndBadIDs(t *testing.T) {
	s := NewFileStorage(testStorageCfg(t.TempDir()), nil)
	for _, bad := range []string{"../../etc/passwd", "mfABCDEF0123456789", "", "mf", "x"} {
		if _, _, err := s.Get(42, bad); err == nil {
			t.Fatalf("traversal/invalid id %q should be rejected", bad)
		}
	}
}

func TestRetentionGC(t *testing.T) {
	root := t.TempDir()
	s := NewFileStorage(testStorageCfg(root), nil)
	raw := pngBytes()
	s.EnqueueFromResult(map[string]any{"data": []any{map[string]any{"b64_json": base64Std(raw)}}}, 42, "m", "")
	waitFor(t, 5*time.Second, func() bool { return len(s.List(42)) == 1 })

	// 把过期时间改到过去。
	meta := s.List(42)[0]
	meta.ExpiresAt = time.Now().Add(-time.Minute)
	buf, _ := json.Marshal(meta)
	_ = os.WriteFile(filepath.Join(root, meta.UserKey, meta.FileID+".json"), buf, 0o600)

	s.gcOnce()
	if files := s.List(42); len(files) != 0 {
		t.Fatalf("expired file not collected: %+v", files)
	}
}

func TestDownloadSizeCap(t *testing.T) {
	root := t.TempDir()
	cfg := testStorageCfg(root)
	cfg.MaxDownloadBytes = 1024
	s := NewFileStorage(cfg, nil)

	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte{1}, 4096))
	}))
	t.Cleanup(big.Close)

	s.EnqueueFromResult(map[string]any{"data": []any{map[string]any{"url": big.URL}}}, 42, "m", "")
	waitFor(t, 5*time.Second, func() bool {
		files := s.List(42)
		return len(files) == 1 && files[0].Status == StatusFailed
	})
	if !strings.Contains(s.List(42)[0].Error, "max size") {
		t.Fatalf("error = %q", s.List(42)[0].Error)
	}
}

func TestUploadListDelete(t *testing.T) {
	s := NewFileStorage(testStorageCfg(t.TempDir()), nil)

	fh := multipartFileHeader(t, "photo.png", "image/png", pngBytes())
	meta, err := s.SaveUpload(42, fh)
	if err != nil {
		t.Fatalf("SaveUpload: %v", err)
	}
	if meta.Status != StatusCompleted || meta.Source != SourceUpload || meta.SizeBytes != int64(len(pngBytes())) {
		t.Fatalf("upload meta = %+v", meta)
	}
	_, path, err := s.Get(42, meta.FileID)
	if err != nil {
		t.Fatalf("Get upload: %v", err)
	}
	if !strings.HasSuffix(path, ".png") {
		t.Fatalf("upload path = %q", path)
	}

	// 越权访问别的用户 → 不存在。
	if _, _, err := s.Get(43, meta.FileID); err == nil {
		t.Fatal("cross-user read should fail")
	}
	if _, _, err := s.Get(42, "mf0000000000000000"); err == nil {
		t.Fatal("unknown id should fail")
	}

	if ok, err := s.Delete(42, meta.FileID); !ok || err != nil {
		t.Fatalf("delete = %v, %v", ok, err)
	}
	if _, _, err := s.Get(42, meta.FileID); err == nil {
		t.Fatal("file should be gone")
	}
}

func TestDisabledStorageNoOps(t *testing.T) {
	cfg := testStorageCfg(t.TempDir())
	cfg.Enabled = false
	s := NewFileStorage(cfg, nil)
	if s.Enabled() {
		t.Fatal("should be disabled")
	}
	s.EnqueueFromPayload([]byte(`{"data":[{"url":"http://x/y.png"}]}`), http.Header{}, 42, "m")
	time.Sleep(100 * time.Millisecond)
	if files := s.List(42); len(files) != 0 {
		t.Fatalf("disabled storage should not persist: %+v", files)
	}
	if _, err := s.SaveUpload(42, multipartFileHeader(t, "a.png", "image/png", pngBytes())); err == nil {
		t.Fatal("disabled upload should error")
	}
}

// ---- 工具 ----

func base64Std(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func multipartFileHeader(t *testing.T, filename, contentType string, data []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="file"; filename="` + filename + `"`}
	h["Content-Type"] = []string{contentType}
	fw, _ := mw.CreatePart(h)
	_, _ = fw.Write(data)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/upload", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse multipart: %v", err)
	}
	return req.MultipartForm.File["file"][0]
}
