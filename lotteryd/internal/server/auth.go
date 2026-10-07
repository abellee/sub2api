// Package server implements lotteryd's HTTP API.
package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims is the subset of Sub2API JWT claims lotteryd relies on.
type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Exp    int64  `json:"exp"`
	// RegisteredAt 账号注册时间（introspect 模式从 /auth/me 的 created_at 带回；本地模式为 nil）。
	RegisteredAt *time.Time `json:"-"`
}

// ParseJWT validates an HS256-signed Sub2API JWT with the shared secret and
// returns its claims. Expiry is enforced with a 30s leeway.
func ParseJWT(token, secret string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("bad signature encoding: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, errors.New("signature mismatch")
	}
	// Header: only accept HS256.
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("bad header encoding: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerRaw, &header); err != nil || header.Alg != "HS256" {
		return nil, errors.New("unsupported alg")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("bad payload encoding: %w", err)
	}
	// user_id is a JSON number; decode via float64 first.
	var raw struct {
		UserID float64 `json:"user_id"`
		Email  string  `json:"email"`
		Role   string  `json:"role"`
		Exp    float64 `json:"exp"`
	}
	if err := json.Unmarshal(payloadRaw, &raw); err != nil {
		return nil, fmt.Errorf("bad claims: %w", err)
	}
	if raw.Exp > 0 && time.Now().Unix() > int64(raw.Exp)+30 {
		return nil, errors.New("token expired")
	}
	if raw.UserID <= 0 {
		return nil, errors.New("no user_id claim")
	}
	return &Claims{UserID: int64(raw.UserID), Email: raw.Email, Role: raw.Role, Exp: int64(raw.Exp)}, nil
}
