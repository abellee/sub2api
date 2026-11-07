// Package config 从环境变量装配 mediad 配置。
package config

import (
	"os"
	"strconv"
)

// Config 是 mediad 的运行配置。
type Config struct {
	// Listen 是 HTTP 监听地址,默认 ":8090"。
	Listen string
	// ManagementToken 是管理面 Bearer Token;为空时管理 API 整体禁用。
	ManagementToken string
	// RelaySecret 是与 sub2api 共享的中转签名密钥;为空时中转端点整体禁用。
	RelaySecret string
	// DataDir 是状态快照目录,默认 "./data"。
	DataDir         string
	ResultDir       string
	DownloadBaseURL string
	MaxFileBytes    int64
	// PersistCredentials 决定快照是否包含上游凭证(用于重启后继续轮询)。
	PersistCredentials bool
	// MaxSyncSeconds 是同步等待的默认上限。
	MaxSyncSeconds int
	// LogLevel: debug|info|warn|error(当前仅影响是否输出 debug 日志)。
	LogLevel string
}

// FromEnv 读取环境变量并应用默认值。
func FromEnv() Config {
	cfg := Config{
		Listen:             env("MEDIAD_LISTEN", ":8090"),
		ManagementToken:    env("MEDIAD_MANAGEMENT_TOKEN", ""),
		RelaySecret:        env("MEDIAD_RELAY_SECRET", ""),
		DataDir:            env("MEDIAD_DATA_DIR", "./data"),
		ResultDir:          env("MEDIAD_RESULT_DIR", ""),
		DownloadBaseURL:    env("MEDIAD_DOWNLOAD_BASE_URL", ""),
		MaxFileBytes:       int64(envInt("MEDIAD_MAX_FILE_MIB", 512)) << 20,
		PersistCredentials: envBool("MEDIAD_PERSIST_CREDENTIALS", true),
		MaxSyncSeconds:     envInt("MEDIAD_MAX_SYNC_SECONDS", 600),
		LogLevel:           env("MEDIAD_LOG_LEVEL", "info"),
	}
	return cfg
}

func env(key, def string) string {
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
