package mediahub

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

// Prefix 是签名值的固定前缀。算法与 mediad/internal/signature 完全一致:
//
//	canonical = timestamp + "\n" + METHOD + "\n" + path + "\n" + body
//	signature = "sha256=" + hex(HMAC-SHA256(secret, canonical))
const Prefix = "sha256="

// MaxSkew 是允许的时钟偏差。
const MaxSkew = 5 * time.Minute

var (
	ErrBadFormat      = errors.New("mediahub/signature: bad format")
	ErrExpired        = errors.New("mediahub/signature: timestamp outside allowed skew")
	ErrMismatch       = errors.New("mediahub/signature: mismatch")
	ErrEmptySecret    = errors.New("mediahub/signature: empty secret")
	ErrEmptyTimestamp = errors.New("mediahub/signature: empty timestamp")
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

// Sign 计算签名值(含前缀)。
func Sign(secret, timestamp, method, path string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(Canonical(timestamp, method, path, body)))
	return Prefix + hex.EncodeToString(mac.Sum(nil))
}

// Verify 校验签名与时效。now 供测试注入。
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
