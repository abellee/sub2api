package signature

import (
	"strings"
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := "shared-secret"
	ts := "1700000000"
	body := []byte(`{"prompt":"hi"}`)
	sig := Sign(secret, ts, "POST", "/relay/mj/v1/images/generations", body)

	if err := Verify(secret, ts, "POST", "/relay/mj/v1/images/generations", body, sig, time.Unix(1700000000, 0)); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	secret := "shared-secret"
	ts := "1700000000"
	body := []byte(`{"prompt":"hi"}`)
	sig := Sign(secret, ts, "POST", "/relay/mj", body)
	now := time.Unix(1700000000, 0)

	cases := []struct {
		name                       string
		modSecret, modTS           string
		modMethod, modPath, modSig string
		modBody                    []byte
	}{
		{"wrong secret", "other", ts, "POST", "/relay/mj", sig, body},
		{"wrong body", secret, ts, "POST", "/relay/mj", sig, []byte(`{"prompt":"evil"}`)},
		{"wrong path", secret, ts, "POST", "/relay/other", sig, body},
		{"wrong method", secret, ts, "GET", "/relay/mj", sig, body},
		{"garbage sig", secret, ts, "POST", "/relay/mj", "garbage", body},
		{"no prefix", secret, ts, "POST", "/relay/mj", strings.TrimPrefix(sig, Prefix), body},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Verify(tc.modSecret, tc.modTS, tc.modMethod, tc.modPath, tc.modBody, tc.modSig, now); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestVerifySkew(t *testing.T) {
	secret, body := "s", []byte(`{}`)
	ts := "1700000000"
	sig := Sign(secret, ts, "POST", "/x", body)
	base := time.Unix(1700000000, 0)

	if err := Verify(secret, ts, "POST", "/x", body, sig, base.Add(MaxSkew-time.Second)); err != nil {
		t.Fatalf("within skew should pass: %v", err)
	}
	if err := Verify(secret, ts, "POST", "/x", body, sig, base.Add(MaxSkew+time.Minute)); err != ErrExpired {
		t.Fatalf("beyond skew should be ErrExpired, got %v", err)
	}
}

func TestVerifyEmptyInputs(t *testing.T) {
	now := time.Now()
	if err := Verify("", "1", "POST", "/x", nil, Prefix+"00", now); err != ErrEmptySecret {
		t.Fatalf("want ErrEmptySecret, got %v", err)
	}
	if err := Verify("s", "", "POST", "/x", nil, Prefix+"00", now); err != ErrEmptyTimestamp {
		t.Fatalf("want ErrEmptyTimestamp, got %v", err)
	}
}
