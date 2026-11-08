package relay

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResultFile points to retained files on the media service; the main site stores only download metadata.
type ResultFile struct {
	DownloadURL string `json:"download_url,omitempty"`
	ID          string `json:"file_id"`
	Path        string `json:"path"`
	Size        int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
}

func downloadFilename(file ResultFile) string {
	ct, _, _ := mime.ParseMediaType(file.ContentType)
	ext := map[string]string{
		"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp",
		"image/gif": ".gif", "image/svg+xml": ".svg", "image/avif": ".avif",
		"video/mp4": ".mp4", "video/webm": ".webm", "audio/mpeg": ".mp3",
		"audio/wav": ".wav", "audio/ogg": ".ogg",
	}[ct]
	if ext == "" {
		ext = ".bin"
	}
	return file.ID + ext
}

// Keep the storage ID and signature stable so existing links remain valid.
func formattedDownloadURL(file ResultFile) string {
	u, err := url.Parse(file.DownloadURL)
	if err != nil || file.DownloadURL == "" {
		return file.DownloadURL
	}
	if strings.HasSuffix(u.Path, "/"+file.ID) {
		u.Path = strings.TrimSuffix(u.Path, file.ID) + downloadFilename(file)
	}
	return u.String()
}

func downloadToken(task *Task, fileID string) string {
	h := hmac.New(sha256.New, []byte(task.DownloadToken))
	h.Write([]byte(task.ID + "/" + fileID))
	return hex.EncodeToString(h.Sum(nil))
}

func (t *Tracker) prepareDownloadLinks(task *Task) error {
	if len(task.Files) == 0 {
		return nil
	}
	if task.DownloadToken == "" {
		var token [32]byte
		if _, err := rand.Read(token[:]); err != nil {
			return err
		}
		task.DownloadToken = hex.EncodeToString(token[:])
	}
	for i := range task.Files {
		task.Files[i].DownloadURL = strings.TrimRight(t.downloadBase, "/") + "/downloads/" + task.ID + "/" + downloadFilename(task.Files[i]) + "?token=" + downloadToken(task, task.Files[i].ID)
	}
	if !t.saveResult(task) {
		return fmt.Errorf("could not persist download links")
	}
	return nil
}

func (h *RelayHandler) handleDownload(w http.ResponseWriter, r *http.Request) {
	publicMediaHeaders(w)
	task, ok := h.store.PublicTask(r.PathValue("taskID"))
	if !ok || task.Status != StatusSucceeded || task.CallbackState != CallbackConfirmed || task.DownloadToken == "" || task.FilesAcknowledged || h.tracker.stager == nil {
		http.NotFound(w, r)
		return
	}
	id := r.PathValue("fileID")
	for _, meta := range task.Files {
		if meta.ID == id || downloadFilename(meta) == id {
			if !hmac.Equal([]byte(downloadToken(task, meta.ID)), []byte(r.URL.Query().Get("token"))) {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", meta.ContentType)
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": downloadFilename(meta)}))
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeFile(w, r, filepath.Join(h.tracker.stager.Root, task.ID, meta.ID))
			return
		}
	}
	http.NotFound(w, r)
}

type ResultStager struct {
	Root     string
	MaxBytes int64
	Client   *http.Client
}

// Stage 只读取 schema 声明的结果字段，按确定顺序落盘，重试复用已有文件。
func (s *ResultStager) Stage(ctx context.Context, task *Task) ([]ResultFile, error) {
	refs := []string{}
	collectResultRefs(task.Result, "", &refs)
	if len(refs) == 0 {
		return nil, fmt.Errorf("no result files; check response.result mapping")
	}
	files := make([]ResultFile, 0, len(refs))
	for i, ref := range refs {
		id := fmt.Sprintf("f%04d", i)
		path := filepath.Join(s.Root, task.ID, id)
		var meta ResultFile
		if buf, err := os.ReadFile(path + ".json"); err == nil && json.Unmarshal(buf, &meta) == nil {
			if f, err := os.Open(path); err == nil {
				h := sha256.New()
				n, readErr := io.Copy(h, f)
				f.Close()
				if readErr == nil && n == meta.Size && hex.EncodeToString(h.Sum(nil)) == meta.SHA256 {
					files = append(files, meta)
					continue
				}
			}
		}
		meta, err := s.save(ctx, task, id, ref)
		if err != nil {
			return nil, err
		}
		files = append(files, meta)
	}
	return files, nil
}

func collectResultRefs(v any, key string, out *[]string) {
	switch n := v.(type) {
	case map[string]any:
		// Gemini returns base64 and its MIME type together, possibly after text parts.
		if key == "inlineData" || key == "inline_data" {
			data, _ := n["data"].(string)
			mime, _ := n["mimeType"].(string)
			if mime == "" {
				mime, _ = n["mime_type"].(string)
			}
			if data != "" && strings.HasPrefix(mime, "image/") {
				*out = append(*out, "data:"+mime+";base64,"+data)
			}
			return
		}
		keys := make([]string, 0, len(n))
		for k := range n {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			collectResultRefs(n[k], k, out)
		}
	case []any:
		for _, e := range n {
			collectResultRefs(e, key, out)
		}
	case string:
		if key == "text" || key == "revised_prompt" {
			return
		}
		if key == "b64_json" {
			*out = append(*out, "data:application/octet-stream;base64,"+n)
		} else if strings.HasPrefix(n, "http://") || strings.HasPrefix(n, "https://") || strings.HasPrefix(n, "data:") || strings.HasPrefix(n, "/") {
			*out = append(*out, n)
		}
	}
}

func (s *ResultStager) save(ctx context.Context, task *Task, id, ref string) (ResultFile, error) {
	var reader io.ReadCloser
	ct := "application/octet-stream"
	if strings.HasPrefix(ref, "data:") {
		parts := strings.SplitN(ref, ",", 2)
		if len(parts) != 2 || !strings.HasSuffix(parts[0], ";base64") {
			return ResultFile{}, fmt.Errorf("invalid data URL")
		}
		ct = strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")
		reader = io.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(parts[1])))
	} else {
		u, err := url.Parse(ref)
		if err != nil {
			return ResultFile{}, err
		}
		if !u.IsAbs() {
			base, err := url.Parse(task.UpstreamURL)
			if err != nil {
				return ResultFile{}, err
			}
			u = base.ResolveReference(u)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return ResultFile{}, fmt.Errorf("invalid result URL scheme")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return ResultFile{}, err
		}
		// 同一上游的受保护文件接口复用凭证，第三方 CDN 不携带上游密钥。
		if base, err := url.Parse(task.UpstreamURL); err == nil && base.Scheme == u.Scheme && base.Host == u.Host && task.Schema != nil {
			for name, tmpl := range task.Schema.Request.Credential {
				req.Header.Set(name, renderTaskTemplate(tmpl, task))
			}
		}
		client := s.Client
		if client == nil {
			client = http.DefaultClient
		}
		downloadClient := *client
		downloadClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
			if next.URL.Scheme != req.URL.Scheme || next.URL.Host != req.URL.Host {
				if task.Schema != nil {
					for name := range task.Schema.Request.Credential {
						next.Header.Del(name)
					}
				}
			}
			if client.CheckRedirect != nil {
				return client.CheckRedirect(next, via)
			}
			if len(via) >= 10 {
				return fmt.Errorf("too many download redirects")
			}
			return nil
		}
		resp, err := downloadClient.Do(req)
		if err != nil {
			return ResultFile{}, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return ResultFile{}, fmt.Errorf("download status %d", resp.StatusCode)
		}
		reader = resp.Body
		ct = strings.Split(resp.Header.Get("Content-Type"), ";")[0]
	}
	defer reader.Close()
	buffered := bufio.NewReader(reader)
	if ct == "" || ct == "application/octet-stream" {
		sample, _ := buffered.Peek(512)
		if len(sample) > 0 {
			ct = http.DetectContentType(sample)
		}
	}
	dir := filepath.Join(s.Root, task.ID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return ResultFile{}, err
	}
	f, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return ResultFile{}, err
	}
	defer os.Remove(f.Name())
	h := sha256.New()
	limit := s.MaxBytes
	if limit <= 0 {
		limit = 512 << 20
	}
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(buffered, limit+1))
	if err == nil && n == 0 {
		err = fmt.Errorf("empty result file")
	}
	if err == nil && n > limit {
		err = fmt.Errorf("result exceeds maximum file size")
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return ResultFile{}, err
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	meta := ResultFile{ID: id, Path: filepath.ToSlash(filepath.Join(task.ID, id)), Size: n, SHA256: hex.EncodeToString(h.Sum(nil)), ContentType: ct}
	if err := os.Rename(f.Name(), filepath.Join(dir, id)); err != nil {
		return ResultFile{}, err
	}
	if err := WriteState(filepath.Join(dir, id+".json"), meta); err != nil {
		return ResultFile{}, err
	}
	return meta, nil
}

func (h *RelayHandler) ownedTask(r *http.Request) (*Task, bool) {
	task, ok := h.store.Get(r.PathValue("taskID"))
	return task, ok && task.UserID == r.Header.Get(HeaderUserID) && task.KeyID == r.Header.Get(HeaderKeyID)
}

func (h *RelayHandler) handleFile(w http.ResponseWriter, r *http.Request) {
	if h.secret == "" || h.verify(r, nil) != nil {
		writeError(w, 401, "invalid_signature", "invalid signature")
		return
	}
	task, ok := h.ownedTask(r)
	if !ok || h.tracker.stager == nil || task.FilesAcknowledged {
		http.NotFound(w, r)
		return
	}
	for _, meta := range task.Files {
		if meta.ID != r.PathValue("fileID") {
			continue
		}
		w.Header().Set("Content-Type", meta.ContentType)
		http.ServeFile(w, r, filepath.Join(h.tracker.stager.Root, task.ID, meta.ID))
		return
	}
	http.NotFound(w, r)
}

// Legacy acknowledgements cannot delete retained media after the storage responsibility changed.
func (h *RelayHandler) handleFilesAck(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil {
		writeError(w, 400, "bad_request", "invalid body")
		return
	}
	if h.secret == "" || h.verify(r, body) != nil {
		writeError(w, 401, "invalid_signature", "invalid signature")
		return
	}
	writeError(w, 410, "files_retained", "files remain on the media service; acknowledgement no longer deletes files")
}
