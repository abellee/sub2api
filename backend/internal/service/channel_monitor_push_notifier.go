package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultChannelMonitorPushURL = "http://127.0.0.1:8091/push-api/v1/internal/channel-check-completed"
	channelMonitorPushTimeout    = 5 * time.Second
)

type channelMonitorCompletionNotifier interface {
	NotifyChannelCheckCompleted(
		ctx context.Context,
		monitorID int64,
		groupName string,
		recentStatuses []string,
		currentStatus string,
	) error
}

type channelMonitorPushNotifier struct {
	endpoint string
	token    string
	client   *http.Client
}

type channelMonitorPushPayload struct {
	MonitorID      int64    `json:"monitorID"`
	GroupName      string   `json:"groupName"`
	RecentStatuses []string `json:"recentStatuses"`
	CurrentStatus  string   `json:"currentStatus"`
}

func newChannelMonitorPushNotifierFromEnv() channelMonitorCompletionNotifier {
	token := strings.TrimSpace(os.Getenv("PUSH_NOTIFIER_INTERNAL_TOKEN"))
	if token == "" {
		return nil
	}
	endpoint := strings.TrimSpace(os.Getenv("PUSH_NOTIFIER_INTERNAL_URL"))
	if endpoint == "" {
		endpoint = defaultChannelMonitorPushURL
	}
	return newChannelMonitorPushNotifier(endpoint, token, &http.Client{Timeout: channelMonitorPushTimeout})
}

func newChannelMonitorPushNotifier(endpoint, token string, client *http.Client) *channelMonitorPushNotifier {
	if client == nil {
		client = &http.Client{Timeout: channelMonitorPushTimeout}
	}
	return &channelMonitorPushNotifier{
		endpoint: strings.TrimSpace(endpoint),
		token:    strings.TrimSpace(token),
		client:   client,
	}
}

func (n *channelMonitorPushNotifier) NotifyChannelCheckCompleted(
	ctx context.Context,
	monitorID int64,
	groupName string,
	recentStatuses []string,
	currentStatus string,
) error {
	if n == nil || n.endpoint == "" || n.token == "" {
		return nil
	}
	payload := channelMonitorPushPayload{
		MonitorID:      monitorID,
		GroupName:      strings.TrimSpace(groupName),
		RecentStatuses: recentStatuses,
		CurrentStatus:  currentStatus,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal channel monitor push payload: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create channel monitor push request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+n.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := n.client.Do(request)
	if err != nil {
		return fmt.Errorf("send channel monitor push request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("channel monitor push returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	return nil
}
