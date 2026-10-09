// Package push calls the standalone push notifier.
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client sends targeted notifications to users who currently have push enabled.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a client. An empty token leaves it unconfigured.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

// Configured reports whether a base URL and token are both present.
func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.token != ""
}

// Message is one notification. A nil UserIDs slice sends to every enabled subscription.
type Message struct {
	Title     string
	Body      string
	URL       string
	DedupeKey string
	UserIDs   []int64
}

// Result is the notifier's delivery summary. Notified counts distinct users.
type Result struct {
	Notified  int  `json:"notified"`
	Delivered int  `json:"delivered"`
	Failed    int  `json:"failed"`
	Deduped   bool `json:"deduped"`
}

type requestBody struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	URL       string   `json:"url"`
	DedupeKey string   `json:"dedupeKey,omitempty"`
	UserIDs   *[]int64 `json:"userIDs,omitempty"`
}

// Subscriber is one user who can currently receive a push.
type Subscriber struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

// ListSubscribers returns users with an enabled, deliverable subscription.
func (c *Client) ListSubscribers(ctx context.Context) ([]Subscriber, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("push notifier is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/push-api/v1/internal/subscribers", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Push-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("push notifier status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var body struct {
		Users []Subscriber `json:"users"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	if body.Users == nil {
		body.Users = []Subscriber{}
	}
	return body.Users, nil
}

// Notify posts one notification. A stored dedupe key returns the previous counts.
func (c *Client) Notify(ctx context.Context, message Message) (Result, error) {
	if !c.Configured() {
		return Result{}, fmt.Errorf("push notifier is not configured")
	}
	body := requestBody{
		Title:     message.Title,
		Body:      message.Body,
		URL:       message.URL,
		DedupeKey: message.DedupeKey,
	}
	if message.UserIDs != nil {
		ids := make([]int64, len(message.UserIDs))
		copy(ids, message.UserIDs)
		body.UserIDs = &ids
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/push-api/v1/internal/notify", bytes.NewReader(raw))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Push-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Result{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("push notifier status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var result Result
	if err := json.Unmarshal(payload, &result); err != nil {
		return Result{}, err
	}
	return result, nil
}
