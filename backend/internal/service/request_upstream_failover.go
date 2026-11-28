package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
)

// Paths and credentials do not make accounts on the same upstream independent.
func accountFailoverUpstream(account *Account) string {
	base := strings.TrimSpace(account.GetCredential("base_url"))
	if account.IsCustomBaseURLEnabled() {
		base = strings.TrimSpace(account.GetCustomBaseURL())
	}
	if base == "" {
		switch account.Platform {
		case PlatformAnthropic:
			base = "https://api.anthropic.com"
		case PlatformGemini:
			base = geminicli.AIStudioBaseURL
			if account.Type == AccountTypeOAuth {
				base = geminicli.GeminiCliBaseURL
			}
		case PlatformGrok:
			base = account.GetGrokBaseURL()
		case PlatformTypeSafe:
			base = account.GetTypeSafeBaseURL()
		case PlatformOpenAI:
			base = account.GetOpenAIBaseURL()
			if account.Type == AccountTypeOAuth {
				base = "https://chatgpt.com"
			}
		default:
			base = account.GetOpenAIBaseURL()
		}
	}
	if base == "" {
		return "platform:" + account.Platform + ":" + account.Type
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Hostname() == "" {
		// Invalid endpoints must not collapse unrelated accounts into one upstream.
		return "account:" + strconv.FormatInt(account.ID, 10)
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	port := parsed.Port()
	if port == "80" && strings.EqualFold(parsed.Scheme, "http") || port == "443" && strings.EqualFold(parsed.Scheme, "https") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	return "host:" + host
}

func withRequestAccountFailoverAccounts(ctx context.Context, accounts []Account) context.Context {
	ctx = withRequestAccountFailoverCount(ctx, len(accounts))
	trace, _ := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	if trace != nil {
		trace.upstreams = make(map[int64]string, len(accounts))
		trace.credentials = make(map[int64]string, len(accounts))
		for i := range accounts {
			trace.upstreams[accounts[i].ID] = accountFailoverUpstream(&accounts[i])
			trace.credentials[accounts[i].ID] = accountFailoverCredential(&accounts[i])
		}
	}
	return ctx
}

// Keep only fingerprints in the request trace, never the credentials themselves.
func accountFailoverCredential(account *Account) string {
	if key := strings.TrimSpace(account.GetCredential("api_key")); key != "" {
		hash := sha256.Sum256([]byte(key))
		return hex.EncodeToString(hash[:])
	}
	return ""
}

func requestAccountFailoverFinalAttempt(ctx context.Context, accountID int64) bool {
	trace, _ := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	if trace == nil || accountID <= 0 {
		return false
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()
	return trace.finalAccountID == accountID
}

type upstreamFailoverSelectionKey struct{}

// Keep the existing scheduler's eligibility, priority and slot handling within
// each upstream round. Unavailable rounds fall through without making requests.
func selectAccountByFailoverUpstream(ctx context.Context, excluded map[int64]struct{}, selectAccount func(context.Context, map[int64]struct{}) (*AccountSelectionResult, error)) (*AccountSelectionResult, error) {
	trace, _ := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	if trace == nil || !RequestAccountFailoverEnabled(ctx) || ctx.Value(upstreamFailoverSelectionKey{}) != nil {
		return selectAccount(ctx, excluded)
	}
	trace.mu.Lock()
	if len(trace.upstreams) == 0 {
		trace.mu.Unlock()
		return selectAccount(ctx, excluded)
	}
	failed := make(map[int64]struct{}, len(trace.FailedAccountIDs))
	for _, id := range trace.FailedAccountIDs {
		failed[id] = struct{}{}
	}
	for _, attempt := range trace.Attempts {
		if attempt.Result == "failed" {
			failed[attempt.AccountID] = struct{}{}
		}
	}
	_, finalFailed := failed[trace.finalAccountID]
	if finalFailed {
		trace.mu.Unlock()
		return nil, ErrNoAvailableAccounts
	}
	counts := make(map[string]int)
	credentialCounts := make(map[string]int)
	credentials := make(map[int64]string, len(trace.credentials))
	for id, key := range trace.credentials {
		credentials[id] = key
	}
	remaining := make(map[int64]string, len(trace.upstreams))
	for id, upstream := range trace.upstreams {
		if _, ok := failed[id]; ok {
			counts[upstream]++
			credentialCounts[upstream+"/"+credentials[id]]++
			continue
		}
		if _, ok := excluded[id]; !ok {
			remaining[id] = upstream
		}
	}
	trace.mu.Unlock()

	ctx = context.WithValue(ctx, upstreamFailoverSelectionKey{}, true)
	for len(remaining) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		upstreams := make(map[string]struct{})
		keys := make(map[string]struct{})
		minimumCredentialCounts := make(map[string]int)
		minimum := int(^uint(0) >> 1)
		for id, upstream := range remaining {
			upstreams[upstream] = struct{}{}
			keys[credentials[id]] = struct{}{}
			minimum = min(minimum, counts[upstream])
			count := credentialCounts[upstream+"/"+credentials[id]]
			if previous, ok := minimumCredentialCounts[upstream]; !ok || count < previous {
				minimumCredentialCounts[upstream] = count
			}
		}
		preferred := make(map[int64]struct{})
		for id, upstream := range remaining {
			if counts[upstream] == minimum && credentialCounts[upstream+"/"+credentials[id]] == minimumCredentialCounts[upstream] {
				preferred[id] = struct{}{}
			}
		}
		passExcluded := cloneExcludedAccountIDs(excluded)
		if passExcluded == nil {
			passExcluded = make(map[int64]struct{})
		}
		trace.mu.Lock()
		for id := range trace.upstreams {
			if _, ok := preferred[id]; !ok {
				passExcluded[id] = struct{}{}
			}
		}
		trace.mu.Unlock()
		selection, err := selectAccount(ctx, passExcluded)
		if err == nil && selection != nil && selection.Account != nil {
			trace.mu.Lock()
			trace.finalAccountID = 0
			upstream, known := remaining[selection.Account.ID]
			if known && len(upstreams) == 1 && len(keys) == 1 && credentialCounts[upstream+"/"+credentials[selection.Account.ID]] > 0 {
				trace.finalAccountID = selection.Account.ID
			}
			trace.upstreams[selection.Account.ID] = accountFailoverUpstream(selection.Account)
			trace.credentials[selection.Account.ID] = accountFailoverCredential(selection.Account)
			trace.mu.Unlock()
			return selection, nil
		}
		if err != nil && !errors.Is(err, ErrNoAvailableAccounts) && !errors.Is(err, ErrNoAvailableCompactAccounts) {
			return selection, err
		}
		for id := range preferred {
			delete(remaining, id)
		}
		if len(remaining) == 0 {
			if err == nil {
				err = ErrNoAvailableAccounts
			}
			return selection, err
		}
	}
	return nil, ErrNoAvailableAccounts
}
