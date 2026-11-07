package mediahub

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

// Settings contains main-site settings; provider accounts and prices remain in their existing pages.
type Settings struct {
	Enabled           bool   `json:"enabled"`
	MediadBaseURL     string `json:"mediad_base_url"`
	ManagementToken   string `json:"management_token,omitempty"`
	RelaySecret       string `json:"relay_secret,omitempty"`
	CallbackSecret    string `json:"callback_secret,omitempty"`
	StorePath         string `json:"bindings_path"`
	ForwardSeconds    int    `json:"forward_timeout_seconds"`
	ManagementSeconds int    `json:"management_timeout_seconds"`
}

func (s Settings) Validate() error {
	if s.MediadBaseURL != "" {
		u, err := url.Parse(s.MediadBaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("微服务地址必须是 HTTP 或 HTTPS 地址，不能含账号、查询参数或片段")
		}
	}
	if s.Enabled && (s.MediadBaseURL == "" || s.RelaySecret == "" || s.CallbackSecret == "") {
		return fmt.Errorf("启用前请填写微服务地址、中转密钥、通知密钥")
	}
	if s.StorePath == "" {
		return fmt.Errorf("绑定文件路径不能为空")
	}
	for _, v := range []struct {
		name            string
		value, min, max int
	}{
		{"请求等待秒数", s.ForwardSeconds, 1, 86400}, {"管理接口等待秒数", s.ManagementSeconds, 1, 600},
	} {
		if v.value < v.min || v.value > v.max {
			return fmt.Errorf("%s须在 %d 到 %d 之间", v.name, v.min, v.max)
		}
	}
	return nil
}

type SettingsStore struct {
	mu              sync.RWMutex
	path            string
	active, desired Settings
}

func OpenSettings(path string, defaults Settings) (*SettingsStore, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(b, &defaults); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := defaults.Validate(); err != nil {
		return nil, err
	}
	return &SettingsStore{path: path, active: defaults, desired: defaults}, nil
}
func (s *SettingsStore) Active() Settings  { s.mu.RLock(); defer s.mu.RUnlock(); return s.active }
func (s *SettingsStore) Desired() Settings { s.mu.RLock(); defer s.mu.RUnlock(); return s.desired }

// Update persists first. Active relay/storage settings are changed only by a restart.
func (s *SettingsStore) Update(next Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if next.ManagementToken == "" {
		next.ManagementToken = s.desired.ManagementToken
	}
	if next.RelaySecret == "" {
		next.RelaySecret = s.desired.RelaySecret
	}
	if next.CallbackSecret == "" {
		next.CallbackSecret = s.desired.CallbackSecret
	}
	if err := next.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".media-settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	s.desired = next
	return nil
}
func (s *SettingsStore) View() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	redact := func(v Settings) map[string]any {
		b, _ := json.Marshal(v)
		out := map[string]any{}
		_ = json.Unmarshal(b, &out)
		for _, key := range []string{"management_token", "relay_secret", "callback_secret"} {
			out[key+"_configured"] = out[key] != nil && out[key] != ""
			delete(out, key)
		}
		return out
	}
	pending := []string{}
	a, b := reflect.ValueOf(s.active), reflect.ValueOf(s.desired)
	for i := 0; i < a.NumField(); i++ {
		if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
			pending = append(pending, strings.Split(a.Type().Field(i).Tag.Get("json"), ",")[0])
		}
	}
	return map[string]any{"settings": redact(s.desired), "active": redact(s.active), "restart_required": len(pending) > 0, "pending_fields": pending}
}
