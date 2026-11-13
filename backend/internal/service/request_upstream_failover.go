package service

import (
	"context"
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
		for i := range accounts {
			trace.upstreams[accounts[i].ID] = accountFailoverUpstream(&accounts[i])
		}
	}
	return ctx
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
	remaining := make(map[int64]string, len(trace.upstreams))
	for id, upstream := range trace.upstreams {
		if _, ok := failed[id]; ok {
			counts[upstream]++
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
		minimum := int(^uint(0) >> 1)
		for _, upstream := range remaining {
			upstreams[upstream] = struct{}{}
			minimum = min(minimum, counts[upstream])
		}
		preferred := make(map[int64]struct{})
		for id, upstream := range remaining {
			if counts[upstream] == minimum {
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
			if known && len(upstreams) == 1 && counts[upstream] > 0 {
				trace.finalAccountID = selection.Account.ID
			}
			trace.upstreams[selection.Account.ID] = accountFailoverUpstream(selection.Account)
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
