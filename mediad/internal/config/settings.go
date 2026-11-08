package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

// Settings covers all service settings. Location of this file is the only bootstrap setting.
type Settings struct {
	Listen                 string `json:"listen"`
	DataDir                string `json:"data_dir"`
	ResultDir              string `json:"result_dir"`
	DownloadBaseURL        string `json:"download_base_url"`
	MaxFileMiB             int    `json:"max_file_mib"`
	MaxSyncSeconds         int    `json:"max_sync_seconds"`
	MaxConcurrent          int    `json:"max_concurrent"`
	PollIntervalSeconds    int    `json:"poll_interval_seconds"`
	MaxPollSeconds         int    `json:"max_poll_seconds"`
	DownloadTimeoutSeconds int    `json:"download_timeout_seconds"`
	DownloadAttempts       int    `json:"download_attempts"`
	TaskRetentionHours     int    `json:"task_retention_hours"`
	FileRetentionHours     int    `json:"file_retention_hours"`
	GCMinutes              int    `json:"gc_minutes"`
	PersistCredentials     bool   `json:"persist_credentials"`
	LogLevel               string `json:"log_level"`
	RelaySecret            string `json:"relay_secret,omitempty"`
	ManagementToken        string `json:"management_token,omitempty"`
}

func DefaultSettings(cfg Config) Settings {
	if cfg.Listen == "" {
		cfg.Listen = ":8090"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	root := cfg.ResultDir
	if root == "" {
		root = filepath.Join(cfg.DataDir, "results")
	}
	maxFile := int(cfg.MaxFileBytes >> 20)
	if maxFile <= 0 {
		maxFile = 512
	}
	maxSync := cfg.MaxSyncSeconds
	if maxSync <= 0 {
		maxSync = 600
	}
	return Settings{Listen: cfg.Listen, DataDir: cfg.DataDir, ResultDir: root, DownloadBaseURL: cfg.DownloadBaseURL, MaxFileMiB: maxFile, MaxSyncSeconds: maxSync, MaxConcurrent: 8, PollIntervalSeconds: 5, MaxPollSeconds: 1800, DownloadTimeoutSeconds: 120, DownloadAttempts: 3, TaskRetentionHours: 24, FileRetentionHours: 1, GCMinutes: 30, PersistCredentials: cfg.PersistCredentials, LogLevel: cfg.LogLevel, RelaySecret: cfg.RelaySecret, ManagementToken: cfg.ManagementToken}
}

func (s Settings) Validate() error {
	if s.DownloadBaseURL != "" {
		u, err := url.Parse(s.DownloadBaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("download_base_url must be an HTTP(S) origin or path prefix")
		}
	}
	if _, _, err := net.SplitHostPort(s.Listen); err != nil {
		return fmt.Errorf("listen must be host:port")
	}
	if s.DataDir == "" || s.ResultDir == "" {
		return fmt.Errorf("data_dir and result_dir are required")
	}
	if !s.PersistCredentials {
		return fmt.Errorf("persist_credentials must remain enabled for reliable task recovery")
	}
	if s.FileRetentionHours < 0 {
		return fmt.Errorf("file_retention_hours must be non-negative")
	}
	limits := []struct {
		name            string
		value, min, max int
	}{
		{"max_file_mib", s.MaxFileMiB, 1, 10240}, {"max_sync_seconds", s.MaxSyncSeconds, 1, 86400},
		{"max_concurrent", s.MaxConcurrent, 1, 128}, {"poll_interval_seconds", s.PollIntervalSeconds, 1, 3600},
		{"max_poll_seconds", s.MaxPollSeconds, 1, 604800}, {"download_timeout_seconds", s.DownloadTimeoutSeconds, 1, 86400},
		{"download_attempts", s.DownloadAttempts, 1, 100}, {"task_retention_hours", s.TaskRetentionHours, 1, 87600}, {"gc_minutes", s.GCMinutes, 1, 1440},
	}
	for _, x := range limits {
		if x.value < x.min || x.value > x.max {
			return fmt.Errorf("%s must be between %d and %d", x.name, x.min, x.max)
		}
	}
	switch s.LogLevel {
	case "", "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log_level")
	}
	return nil
}

type SettingsStore struct {
	mu              sync.RWMutex
	path            string
	active, desired Settings
}

func OpenSettings(path string, cfg Config) (*SettingsStore, error) {
	s := DefaultSettings(cfg)
	buf, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(buf, &s); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &SettingsStore{path: path, active: s, desired: s}, nil
}

func (s *SettingsStore) Active() Settings { s.mu.RLock(); defer s.mu.RUnlock(); return s.active }
func (s *SettingsStore) StorageChangePending() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active.DataDir != s.desired.DataDir || s.active.ResultDir != s.desired.ResultDir
}

// Runtime configuration changes take effect together on restart, without mutating active workers.
func (s *SettingsStore) Update(next Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if next.RelaySecret == "" {
		next.RelaySecret = s.desired.RelaySecret
	}
	if next.ManagementToken == "" {
		next.ManagementToken = s.desired.ManagementToken
	}
	if err := next.Validate(); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(buf); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	s.desired = next
	// Retention only affects completed tasks and can safely change without restarting workers.
	s.active.FileRetentionHours = next.FileRetentionHours
	return nil
}

func (s *SettingsStore) FileRetentionHours() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active.FileRetentionHours
}

func (s *SettingsStore) View() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	redact := func(v Settings) map[string]any {
		buf, _ := json.Marshal(v)
		out := map[string]any{}
		json.Unmarshal(buf, &out)
		delete(out, "relay_secret")
		delete(out, "management_token")
		out["relay_secret_configured"] = v.RelaySecret != ""
		out["management_token_configured"] = v.ManagementToken != ""
		return out
	}
	pending := []string{}
	a, b := reflect.ValueOf(s.active), reflect.ValueOf(s.desired)
	typ := a.Type()
	for i := 0; i < a.NumField(); i++ {
		if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
			pending = append(pending, strings.Split(typ.Field(i).Tag.Get("json"), ",")[0])
		}
	}
	return map[string]any{"settings": redact(s.desired), "active": redact(s.active), "restart_required": len(pending) > 0, "pending_fields": pending}
}

func (s Settings) Config() Config {
	return Config{Listen: s.Listen, DataDir: s.DataDir, ResultDir: s.ResultDir, DownloadBaseURL: s.DownloadBaseURL, MaxFileBytes: int64(s.MaxFileMiB) << 20, MaxSyncSeconds: s.MaxSyncSeconds, PersistCredentials: s.PersistCredentials, LogLevel: s.LogLevel, RelaySecret: s.RelaySecret, ManagementToken: s.ManagementToken}
}
