package mediahub

// 结果文件落盘存储:
// - 目录按"不可变用户标识"命名(32 位随机 hex,首次分配永久不变),
//   用户的数字 ID 绝不出现在任何文件路径中;
// - 下载/清理全部在协程中执行,不阻塞 sub2api 主业务;
// - 存储目录不提供任何静态索引,文件只能经鉴权 API 逐个读取。

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StorageConfig 是文件存储配置(env 装配,见 LoadStorageConfig)。
type StorageConfig struct {
	Enabled             bool
	Root                string
	Retention           time.Duration
	MaxDownloadBytes    int64
	MaxUploadBytes      int64
	GCInterval          time.Duration
	DownloadConcurrency int
}

// LoadStorageConfig 从环境变量装配:
// MEDIAHUB_STORAGE_ENABLED / MEDIAHUB_STORAGE_ROOT /
// MEDIAHUB_STORAGE_RETENTION_HOURS / MEDIAHUB_STORAGE_MAX_DOWNLOAD_BYTES /
// MEDIAHUB_STORAGE_UPLOAD_MAX_BYTES / MEDIAHUB_STORAGE_GC_MINUTES。
func LoadStorageConfig() StorageConfig {
	cfg := StorageConfig{
		Enabled:             envBool("MEDIAHUB_STORAGE_ENABLED", false),
		Root:                envString("MEDIAHUB_STORAGE_ROOT", filepath.Join("data", "mediahub_files")),
		Retention:           time.Duration(envInt("MEDIAHUB_STORAGE_RETENTION_HOURS", 72)) * time.Hour,
		MaxDownloadBytes:    int64(envInt("MEDIAHUB_STORAGE_MAX_DOWNLOAD_BYTES", 512)),
		MaxUploadBytes:      int64(envInt("MEDIAHUB_STORAGE_UPLOAD_MAX_BYTES", 128)),
		GCInterval:          time.Duration(envInt("MEDIAHUB_STORAGE_GC_MINUTES", 10)) * time.Minute,
		DownloadConcurrency: 4,
	}
	cfg.MaxDownloadBytes <<= 20
	cfg.MaxUploadBytes <<= 20
	return cfg
}

// 文件来源。
const (
	SourceGeneration = "generation"
	SourceUpload     = "upload"
)

// 文件状态。
const (
	StatusDownloading = "downloading"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
)

// FileMeta 是文件的完整元数据(读取接口原样返回)。
type FileMeta struct {
	FileID      string    `json:"file_id"`
	UserID      int64     `json:"user_id"`
	UserKey     string    `json:"user_key"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256,omitempty"`
	Source      string    `json:"source"`
	SourceURL   string    `json:"source_url,omitempty"`
	TaskID      string    `json:"task_id,omitempty"`
	Model       string    `json:"model,omitempty"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

var (
	reFileID  = regexp.MustCompile(`^mf[0-9a-f]{16}$`)
	reUserKey = regexp.MustCompile(`^u[0-9a-f]{31}$`)
)

// FileStorage 是落盘存储实现。
type FileStorage struct {
	cfg      StorageConfig
	client   *http.Client
	keysMu   sync.Mutex
	keysPath string
	keys     map[int64]string
	sem      chan struct{}
	logf     func(format string, args ...any)
}

// NewFileStorage 创建存储;root 目录惰性创建。
func NewFileStorage(cfg StorageConfig, logf func(format string, args ...any)) *FileStorage {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &FileStorage{
		cfg:      cfg,
		client:   &http.Client{Timeout: 120 * time.Second},
		keysPath: filepath.Join(cfg.Root, ".user_keys.json"),
		keys:     map[int64]string{},
		sem:      make(chan struct{}, max(1, cfg.DownloadConcurrency)),
		logf:     logf,
	}
}

// Enabled 报告存储是否启用。
func (s *FileStorage) Enabled() bool { return s != nil && s.cfg.Enabled }

// userKey 取/建用户的不可变目录标识。首次分配 32 位随机 hex 并持久化,
// 此后永不改变;数字 ID 不进入目录名。映射文件损坏时报错而不是重分配,
// 避免既有文件因标识重置而"失踪"。
func (s *FileStorage) userKey(userID int64) (string, error) {
	if userID <= 0 {
		return "", errors.New("mediahub/storage: invalid user id")
	}
	s.keysMu.Lock()
	defer s.keysMu.Unlock()
	if key, ok := s.keys[userID]; ok {
		return key, nil
	}
	if len(s.keys) == 0 {
		if err := s.loadKeysLocked(); err != nil {
			return "", fmt.Errorf("mediahub/storage: user key store unreadable: %w", err)
		}
		if key, ok := s.keys[userID]; ok {
			return key, nil
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	key := "u" + hex.EncodeToString(buf)[1:]
	s.keys[userID] = key
	if err := s.saveKeysLocked(); err != nil {
		return "", err
	}
	return key, nil
}

func (s *FileStorage) loadKeysLocked() error {
	buf, err := os.ReadFile(s.keysPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(buf, &s.keys)
}

func (s *FileStorage) saveKeysLocked() error {
	if err := os.MkdirAll(s.cfg.Root, 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(s.keys, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.keysPath + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.keysPath)
}

// userDir 返回用户目录(不存在则创建)。
func (s *FileStorage) userDir(userID int64) (string, string, error) {
	key, err := s.userKey(userID)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(s.cfg.Root, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	return dir, key, nil
}

// EnqueueFromResult 从异步回调的 result 中提取结果文件并入队下载。
// 立即返回,下载在协程中执行。
func (s *FileStorage) EnqueueFromResult(result map[string]any, userID int64, model, taskID string) {
	if !s.Enabled() || len(result) == 0 {
		return
	}
	refs := collectRefs(any(result))
	if len(refs) == 0 {
		return
	}
	s.enqueue(userID, model, taskID, SourceGeneration, refs)
}

// EnqueueFromPayload 从同步响应体与响应头中提取结果并入队。
// 提取与下载全部在协程中执行(响应体可达数十 MB,深度扫描不能占热路径)。
func (s *FileStorage) EnqueueFromPayload(body []byte, header http.Header, userID int64, model string) {
	if !s.Enabled() || len(body) == 0 {
		return
	}
	headerCopy := header.Clone()
	bodyCopy := append([]byte(nil), body...)
	go func() {
		defer panicRecover("mediahub/storage sync-extract")
		var refs []fileRef
		if raw := headerCopy.Get(HeaderResult); raw != "" {
			var m map[string]any
			if json.Unmarshal([]byte(raw), &m) == nil {
				refs = append(refs, collectRefs(any(m))...)
			}
		}
		if len(refs) == 0 {
			var parsed any
			if json.Unmarshal(bodyCopy, &parsed) == nil {
				refs = collectRefs(parsed)
			}
		}
		if len(refs) == 0 {
			return
		}
		for _, ref := range refs {
			s.sem <- struct{}{}
			func() {
				defer func() { <-s.sem }()
				ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
				defer cancel()
				if err := s.persistOne(ctx, userID, model, "", SourceGeneration, ref); err != nil {
					s.logf("[mediahub/storage] persist failed user=%d file=%s: %v", userID, ref.fileID, err)
				}
			}()
		}
	}()
}

func (s *FileStorage) enqueue(userID int64, model, taskID, source string, refs []fileRef) {
	go func() {
		defer panicRecover("mediahub/storage download")
		for _, ref := range refs {
			s.sem <- struct{}{}
			func() {
				defer func() { <-s.sem }()
				ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
				defer cancel()
				if err := s.persistOne(ctx, userID, model, taskID, source, ref); err != nil {
					s.logf("[mediahub/storage] persist failed user=%d file=%s: %v", userID, ref.fileID, err)
				}
			}()
		}
	}()
}

// persistOne 落一个文件:URL 下载或内联数据解码。
func (s *FileStorage) persistOne(ctx context.Context, userID int64, model, taskID, source string, ref fileRef) error {
	dir, key, err := s.userDir(userID)
	if err != nil {
		return err
	}
	meta := &FileMeta{
		FileID:    ref.fileID,
		UserID:    userID,
		UserKey:   key,
		Model:     model,
		TaskID:    taskID,
		Source:    source,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.cfg.Retention),
		Status:    StatusDownloading,
	}
	if err := s.writeMeta(dir, meta); err != nil {
		return err
	}

	var data []byte
	switch ref.kind {
	case refData:
		data = ref.data
		meta.ContentType = ref.contentType
		meta.Filename = ref.filename
	case refURL:
		meta.SourceURL = ref.url
		file, ct, err2 := s.download(ctx, ref.url)
		if err2 != nil {
			meta.Status = StatusFailed
			meta.Error = err2.Error()
			return s.writeMeta(dir, meta)
		}
		data = file
		meta.ContentType = ct
	default:
		return fmt.Errorf("unknown ref kind %q", ref.kind)
	}
	if int64(len(data)) > s.cfg.MaxDownloadBytes {
		meta.Status = StatusFailed
		meta.Error = "file exceeds max size"
		return s.writeMeta(dir, meta)
	}

	name := meta.Filename
	if name == "" {
		name = meta.FileID + extensionFor(meta.ContentType)
	}
	meta.Filename = sanitizeFilename(name)
	meta.SizeBytes = int64(len(data))
	meta.SHA256 = sha256Hex(data)
	meta.Status = StatusCompleted

	// 内容去重:同一用户已存在相同 SHA256 的文件时丢弃本次副本,复用旧文件。
	if dup := s.findDuplicate(dir, meta); dup != "" {
		_ = os.Remove(filepath.Join(dir, meta.FileID+extensionFor(meta.ContentType)))
		_ = os.Remove(filepath.Join(dir, meta.FileID+".json"))
		s.logf("[mediahub/storage] duplicate content skipped user=%d file=%s dup-of=%s", userID, meta.FileID, dup)
		return nil
	}

	final := filepath.Join(dir, meta.FileID+extensionFor(meta.ContentType))
	if err := atomicWrite(final, data); err != nil {
		meta.Status = StatusFailed
		meta.Error = err.Error()
		return s.writeMeta(dir, meta)
	}
	return s.writeMeta(dir, meta)
}

// findDuplicate 返回同用户目录下与 meta.SHA256 相同的既有 file_id(无则空)。
func (s *FileStorage) findDuplicate(dir string, meta *FileMeta) string {
	if meta.SHA256 == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || e.Name() == meta.FileID+".json" {
			continue
		}
		if m, err := readMeta(filepath.Join(dir, e.Name())); err == nil &&
			m.Status == StatusCompleted && m.SHA256 == meta.SHA256 {
			return m.FileID
		}
	}
	return ""
}

// download 抓取 URL,限制大小并探测 Content-Type。
func (s *FileStorage) download(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, s.cfg.MaxDownloadBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > s.cfg.MaxDownloadBytes {
		return nil, "", errors.New("download exceeds max size")
	}
	ct := resp.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(ct)
	if ct == "" || ct == "application/octet-stream" {
		ct = http.DetectContentType(data)
	}
	return data, ct, nil
}

// ---- 元数据与文件操作(同步 API 用,均为快速本地操作) ----

// List 返回用户全部文件元数据(按创建时间倒序)。
func (s *FileStorage) List(userID int64) []FileMeta {
	out := []FileMeta{}
	if !s.Enabled() {
		return out
	}
	dir, _, err := s.userDir(userID)
	if err != nil {
		return out
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if m, err := readMeta(filepath.Join(dir, e.Name())); err == nil && m.UserID == userID {
			out = append(out, *m)
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].CreatedAt.After(out[j-1].CreatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Get 返回文件元数据与磁盘路径;归属校验失败按不存在处理。
func (s *FileStorage) Get(userID int64, fileID string) (*FileMeta, string, error) {
	if !s.Enabled() || !reFileID.MatchString(fileID) {
		return nil, "", os.ErrNotExist
	}
	dir, key, err := s.userDir(userID)
	if err != nil {
		return nil, "", err
	}
	m, err := readMeta(filepath.Join(dir, fileID+".json"))
	if err != nil || m.UserID != userID || m.UserKey != key {
		return nil, "", os.ErrNotExist
	}
	ext := extensionFor(m.ContentType)
	path := filepath.Join(dir, m.FileID+ext)
	if _, err := os.Stat(path); err != nil {
		// 扩展名未知时按文件名匹配兜底。
		alt := filepath.Join(dir, m.Filename)
		if _, err2 := os.Stat(alt); err2 == nil {
			return m, alt, nil
		}
		return nil, "", os.ErrNotExist
	}
	return m, path, nil
}

// Delete 删除文件与元数据;归属不符按不存在处理。
func (s *FileStorage) Delete(userID int64, fileID string) (bool, error) {
	m, path, err := s.Get(userID, fileID)
	if err != nil {
		return false, nil
	}
	dir := filepath.Dir(path)
	_ = os.Remove(path)
	_ = os.Remove(filepath.Join(dir, m.FileID+".json"))
	return true, nil
}

// SaveUpload 保存用户上传的文件(有界同步写:上传本身是用户显式请求,
// 内容直接来自请求体,无网络等待)。
func (s *FileStorage) SaveUpload(userID int64, fh *multipart.FileHeader) (*FileMeta, error) {
	if !s.Enabled() {
		return nil, errors.New("mediahub/storage: storage disabled")
	}
	if fh.Size > s.cfg.MaxUploadBytes {
		return nil, fmt.Errorf("upload exceeds max size %d", s.cfg.MaxUploadBytes)
	}
	dir, key, err := s.userDir(userID)
	if err != nil {
		return nil, err
	}
	src, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, s.cfg.MaxUploadBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > s.cfg.MaxUploadBytes {
		return nil, fmt.Errorf("upload exceeds max size %d", s.cfg.MaxUploadBytes)
	}

	now := time.Now()
	meta := &FileMeta{
		FileID:      newFileID(),
		UserID:      userID,
		UserKey:     key,
		Filename:    sanitizeFilename(fh.Filename),
		ContentType: detectContentType(fh.Header.Get("Content-Type"), data),
		SizeBytes:   int64(len(data)),
		Source:      SourceUpload,
		Status:      StatusCompleted,
		CreatedAt:   now,
		ExpiresAt:   now.Add(s.cfg.Retention),
	}
	meta.SHA256 = sha256Hex(data)
	if err := atomicWrite(filepath.Join(dir, meta.FileID+extensionFor(meta.ContentType)), data); err != nil {
		return nil, err
	}
	if err := s.writeMeta(dir, meta); err != nil {
		return nil, err
	}
	return meta, nil
}

// RunGC 是保留期清理循环:到期文件与元数据一并删除。单独协程调用。
func (s *FileStorage) RunGC(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.GCInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.gcOnce()
		}
	}
}

func (s *FileStorage) gcOnce() {
	entries, err := os.ReadDir(s.cfg.Root)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		if !e.IsDir() || !reUserKey.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(s.cfg.Root, e.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			m, err := readMeta(filepath.Join(dir, f.Name()))
			if err != nil {
				continue
			}
			if now.After(m.ExpiresAt) {
				_, _ = s.Delete(m.UserID, m.FileID)
			}
		}
	}
}

// ---- 内部工具 ----

type refKind string

const (
	refURL  refKind = "url"
	refData refKind = "data"
)

type fileRef struct {
	kind        refKind
	url         string
	data        []byte
	contentType string
	filename    string
	fileID      string
}

// collectRefs 深度遍历 JSON 值,收集结果文件引用:
// - http(s) URL(键名含 url,或不带键名的 URL 字符串);
// - data URL 与 b64_json 字段(内联解码)。
// 上限 64 个,深度 12 层。
func collectRefs(v any) []fileRef {
	var out []fileRef
	seen := 0
	var walk func(node any, key string, depth int)
	walk = func(node any, key string, depth int) {
		if len(out) >= 64 || depth > 12 || seen > 4096 {
			return
		}
		seen++
		switch cur := node.(type) {
		case map[string]any:
			for k, v := range cur {
				walk(v, k, depth+1)
			}
		case []any:
			for _, v := range cur {
				walk(v, key, depth+1)
			}
		case string:
			trimmed := strings.TrimSpace(cur)
			switch {
			case strings.HasPrefix(trimmed, "data:") && strings.Contains(trimmed, "base64,"):
				data, ct, err := decodeDataURL(trimmed)
				if err == nil {
					out = append(out, fileRef{kind: refData, data: data, contentType: ct, fileID: newFileID()})
				}
			case strings.EqualFold(key, "b64_json") || strings.EqualFold(key, "b64"):
				data, err := decodeBase64Loose(trimmed)
				if err == nil && len(data) > 0 {
					out = append(out, fileRef{kind: refData, data: data,
						contentType: http.DetectContentType(data), fileID: newFileID()})
				}
			case isHTTPLike(key, trimmed):
				out = append(out, fileRef{kind: refURL, url: trimmed, fileID: newFileID()})
			}
		}
	}
	walk(v, "", 0)
	return out
}

func isHTTPLike(key, v string) bool {
	lower := strings.ToLower(key)
	return (strings.Contains(lower, "url") || strings.Contains(lower, "image") || strings.Contains(lower, "video")) &&
		(strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://"))
}

func decodeDataURL(s string) ([]byte, string, error) {
	// data:image/png;base64,xxxx
	head := s
	var ct string
	if i := strings.Index(s, ";"); i > 0 {
		ct = s[len("data:"):i]
		head = s[i+1:]
	}
	idx := strings.Index(head, "base64,")
	if idx < 0 {
		return nil, "", errors.New("not base64 data url")
	}
	data, err := decodeBase64Loose(head[idx+len("base64,"):])
	if err != nil {
		return nil, "", err
	}
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	return data, ct, nil
}

func decodeBase64Loose(s string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/', r == '=':
			return r
		}
		return -1
	}, s)
	return base64.StdEncoding.DecodeString(clean)
}

// 环境变量帮助函数(与 handler 桥的配置装配独立,便于 SDK 自包含测试)。
func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func readMeta(path string) (*FileMeta, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m FileMeta
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *FileStorage) writeMeta(dir string, meta *FileMeta) error {
	buf, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, meta.FileID+".json"), buf)
}

// atomicWrite 临时文件 + rename 的原子落盘。
// Windows 上杀软/索引器可能瞬时锁住刚创建的 .tmp 文件导致 rename
// 报 Access denied,做有界重试(指数退避),其他平台重试无副作用。
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(20<<uint(attempt-1)) * time.Millisecond)
		}
		if err := os.WriteFile(tmp, data, 0o600); err != nil {
			lastErr = err
			continue
		}
		if err := os.Rename(tmp, path); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("atomic write %s: %w", path, lastErr)
}

func newFileID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return "mf" + hex.EncodeToString(buf)
}

func extensionFor(contentType string) string {
	switch strings.TrimSpace(strings.ToLower(contentType)) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "application/zip":
		return ".zip"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == "/" || name == "\\" {
		return ""
	}
	if len(name) > 128 {
		name = name[len(name)-128:]
	}
	return name
}

func detectContentType(declared string, data []byte) string {
	if ct := strings.TrimSpace(strings.ToLower(declared)); ct != "" && ct != "application/octet-stream" {
		if i := strings.Index(ct, ";"); i >= 0 {
			ct = ct[:i]
		}
		return ct
	}
	if len(data) > 0 {
		return http.DetectContentType(data)
	}
	return "application/octet-stream"
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func panicRecover(scope string) {
	if r := recover(); r != nil {
		fmt.Printf("[mediahub] panic in %s: %v\n", scope, r)
	}
}
