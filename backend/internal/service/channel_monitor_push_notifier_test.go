//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChannelMonitorPushNotifierSendsCompletionSummary(t *testing.T) {
	var received channelMonitorPushPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer shared-secret" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	notifier := newChannelMonitorPushNotifier(server.URL, "shared-secret", server.Client())
	err := notifier.NotifyChannelCheckCompleted(
		context.Background(),
		"CC-Kiro",
		[]string{"operational", "operational", "degraded", "operational", "failed"},
		"degraded",
	)
	if err != nil {
		t.Fatalf("notify completion: %v", err)
	}
	if received.GroupName != "CC-Kiro" || len(received.RecentStatuses) != 5 || received.CurrentStatus != "degraded" {
		t.Fatalf("unexpected completion summary: %+v", received)
	}
}

func TestChannelMonitorPushNotifierReturnsNonSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	notifier := newChannelMonitorPushNotifier(server.URL, "wrong-secret", server.Client())
	if err := notifier.NotifyChannelCheckCompleted(
		context.Background(), "channel-a", []string{"failed"}, "failed",
	); err == nil {
		t.Fatal("expected non-success response to return an error")
	}
}
