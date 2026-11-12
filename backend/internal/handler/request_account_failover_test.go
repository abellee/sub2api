package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountErrorFailoverTestSettingsRepo struct{ service.SettingRepository }

func (accountErrorFailoverTestSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if key == service.SettingKeyAccountErrorFailoverEnabled {
		return "true", nil
	}
	return "", service.ErrSettingNotFound
}

func (accountErrorFailoverTestSettingsRepo) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	return map[string]string{service.SettingKeyAccountErrorFailoverEnabled: "true"}, nil
}

func newAccountErrorFailoverTestSettings() *service.SettingService {
	return service.NewSettingService(accountErrorFailoverTestSettingsRepo{}, &config.Config{})
}
