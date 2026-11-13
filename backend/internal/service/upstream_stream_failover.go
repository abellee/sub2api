package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
)

// A semantic marker shared by stream readers, independent of wire protocol.
type upstreamStreamError struct{ cause error }

func (e *upstreamStreamError) Error() string { return e.cause.Error() }
func (e *upstreamStreamError) Unwrap() error { return e.cause }

var errUpstreamStreamMissingTerminal = &upstreamStreamError{
	cause: errors.New("stream usage incomplete: missing terminal event"),
}

func upstreamStreamReadError(err error) error {
	return &upstreamStreamError{cause: fmt.Errorf("stream read error: %w", err)}
}

// Failed attempts are billed only for usage already reported by the upstream.
func (r *ForwardResult) HasObservedUsage() bool {
	return r != nil && (r.Usage.hasObservedTokens() || r.ImageCount > 0 || r.SearchCount > 0)
}

func (r *OpenAIForwardResult) HasObservedUsage() bool {
	return r != nil && (r.Usage.InputTokens > 0 || r.Usage.OutputTokens > 0 ||
		r.Usage.CacheReadInputTokens > 0 || r.Usage.CacheCreationInputTokens > 0 ||
		r.Usage.ImageInputTokens > 0 || r.Usage.ImageCacheReadTokens > 0 ||
		r.Usage.ImageOutputTokens > 0 || r.ImageCount > 0 || r.VideoCount > 0 || r.SearchCount > 0 || r.WebSearchCalls > 0)
}

func requestAccountFailoverError(ctx context.Context, err error) error {
	if err == nil || !RequestAccountFailoverEnabled(ctx) || ctx.Err() != nil ||
		errors.Is(err, bufio.ErrTooLong) || errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
		return err
	}
	var failure *UpstreamFailoverError
	if errors.As(err, &failure) {
		return err // Preserve explicit stop decisions and existing error policies.
	}
	var streamErr *upstreamStreamError
	if !errors.As(err, &streamErr) {
		return err
	}
	return &UpstreamFailoverError{
		Cause:                    err,
		StatusCode:               http.StatusBadGateway,
		SafeToFailoverAfterWrite: true,
		SkipAccountCooldown:      true,
		NextAccountAction:        NextAccountRetry,
		ClientStatusCode:         http.StatusBadGateway,
		ClientMessage:            "Upstream response stream was interrupted",
	}
}
