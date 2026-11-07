// Package signature 实现 sub2api ↔ mediad 之间的 HMAC-SHA256 请求/回调签名。
//
// 规范串(canonical string)双方必须完全一致:
//
//	timestamp + "\n" + method + "\n" + path + "\n" + body
//
// - timestamp:Unix 秒字符串;
// - method:大写 HTTP 方法;
// - path:URL 路径(不含 host、不含 query);
// - body:原始字节,GET 等无 body 请求为空。
//
// 签名头携带 "sha256=" + hex(hmac-sha256(secret, canonical))。
package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Prefix 是签名值的固定前缀。
const Prefix = "sha256="

// MaxSkew 是允许的时钟偏差。
const MaxSkew = 5 * time.Minute

var (
	ErrBadFormat      = errors.New("signature: bad format")
	ErrExpired        = errors.New("signature: timestamp outside allowed skew")
	ErrMismatch       = errors.New("signature: mismatch")
	ErrEmptySecret    = errors.New("signature: empty secret")
	ErrEmptyTimestamp = errors.New("signature: empty timestamp")
)

// Canonical 组装规范串。
func Canonical(timestamp, method, path string, body []byte) string {
	var b strings.Builder
	b.WriteString(timestamp)
	b.WriteByte('\n')
	b.WriteString(strings.ToUpper(method))
	b.WriteByte('\n')
	b.WriteString(path)
	b.WriteByte('\n')
	b.Write(body)
	return b.String()
}

// Sign 计算签名值(含 "sha256=" 前缀)。
func Sign(secret, timestamp, method, path string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(Canonical(timestamp, method, path, body)))
	return Prefix + hex.EncodeToString(mac.Sum(nil))
}

// Verify 校验签名值与时间戳时效。now 传入当前时间以便测试注入。
func Verify(secret, timestamp, method, path string, body []byte, sig string, now time.Time) error {
	if strings.TrimSpace(secret) == "" {
		return ErrEmptySecret
	}
	if strings.TrimSpace(timestamp) == "" {
		return ErrEmptyTimestamp
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return fmt.Errorf("%w: bad timestamp", ErrBadFormat)
	}
	if d := now.Sub(time.Unix(ts, 0)); d > MaxSkew || d < -MaxSkew {
		return ErrExpired
	}
	trimmed := strings.TrimSpace(sig)
	if !strings.HasPrefix(trimmed, Prefix) {
		return ErrBadFormat
	}
	got, err := hex.DecodeString(strings.TrimPrefix(trimmed, Prefix))
	if err != nil {
		return fmt.Errorf("%w: bad hex", ErrBadFormat)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(Canonical(timestamp, method, path, body)))
	if !hmac.Equal(got, mac.Sum(nil)) {
		return ErrMismatch
	}
	return nil
}
