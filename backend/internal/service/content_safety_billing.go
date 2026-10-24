package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	_ "modernc.org/sqlite"
)

// Configuration and the durable debit journal live together in SQLite. Balances
// remain in the primary database; its existing atomic billing dedup transaction
// makes replay safe if the process stops between the two database commits.
type ContentSafetyBillingConfig struct {
	Enabled          bool    `json:"enabled"`
	Amount           float64 `json:"amount"`
	MatchDescription string  `json:"match_description"`
	ClientMessage    string  `json:"client_message"`
}

type ContentSafetyDebit struct {
	ID                string  `json:"id"`
	UserID            int64   `json:"user_id"`
	APIKeyID          int64   `json:"api_key_id"`
	AccountID         int64   `json:"account_id"`
	UpstreamRequestID string  `json:"upstream_request_id"`
	Amount            float64 `json:"amount"`
	Notes             string  `json:"notes"`
	ClientMessage     string  `json:"client_message"`
	Status            string  `json:"status"`
	CreatedAt         string  `json:"created_at"`
	ChargedAt         string  `json:"charged_at"`
	LastError         string  `json:"last_error"`
}

type ContentSafetyDebitPage struct {
	Items    []ContentSafetyDebit `json:"items"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

const (
	// ContentSafetyUsageModel is the stable usage-log model users see for a safety debit.
	ContentSafetyUsageModel = "content-safety"
	// ContentSafetyBillingMode marks that usage row as a fixed content-safety charge.
	ContentSafetyBillingMode = string(BillingModeContentSafety)
)

type contentSafetyBillingRepository interface {
	Apply(context.Context, *UsageBillingCommand) (*UsageBillingApplyResult, error)
}

type contentSafetyUsageRecorder interface {
	Create(context.Context, *UsageLog) (bool, error)
}

type ContentSafetyBillingService struct {
	db         *sql.DB
	billing    contentSafetyBillingRepository
	usage      contentSafetyUsageRecorder
	invalidate func(context.Context, int64) error
	processMu  sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}
}

func OpenContentSafetyBilling(path string, billing contentSafetyBillingRepository, usage contentSafetyUsageRecorder, invalidate func(context.Context, int64) error) (*ContentSafetyBillingService, error) {
	// The path comes from operator-managed DATA_DIR/configuration, never a request.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { //nolint:gosec // G703: trusted administrator-configured storage path.
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL;
		PRAGMA busy_timeout=5000;
		PRAGMA synchronous=FULL;
		CREATE TABLE IF NOT EXISTS content_safety_config (
		 id INTEGER PRIMARY KEY CHECK(id=1), enabled INTEGER NOT NULL DEFAULT 0,
		 amount_units INTEGER NOT NULL DEFAULT 0, match_description TEXT NOT NULL,
		 client_message TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS content_safety_debits (
		 id TEXT PRIMARY KEY, user_id INTEGER NOT NULL, api_key_id INTEGER NOT NULL,
		 account_id INTEGER NOT NULL, upstream_request_id TEXT NOT NULL,
		 amount_units INTEGER NOT NULL, notes TEXT NOT NULL, client_message TEXT NOT NULL,
		 status TEXT NOT NULL DEFAULT 'pending', created_at TEXT NOT NULL,
		 charged_at TEXT NOT NULL DEFAULT '', last_error TEXT NOT NULL DEFAULT '',
		 attempted_at TEXT NOT NULL DEFAULT '');
		CREATE INDEX IF NOT EXISTS content_safety_debits_pending ON content_safety_debits(status, attempted_at);
		CREATE INDEX IF NOT EXISTS content_safety_debits_created ON content_safety_debits(created_at DESC);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO content_safety_config(id,match_description,client_message) VALUES(1,?,?)`,
		"I'm sorry, I can't help with that request. Failed check: SAFETY_CHECK_TYPE", "Request rejected by content safety policy.")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &ContentSafetyBillingService{db: db, billing: billing, usage: usage, invalidate: invalidate}, nil
}

func (s *ContentSafetyBillingService) Config(ctx context.Context) (ContentSafetyBillingConfig, error) {
	var c ContentSafetyBillingConfig
	var units int64
	err := s.db.QueryRowContext(ctx, `SELECT enabled, amount_units, match_description, client_message FROM content_safety_config WHERE id=1`).Scan(&c.Enabled, &units, &c.MatchDescription, &c.ClientMessage)
	c.Amount = float64(units) / 1e8
	return c, err
}

func (c ContentSafetyBillingConfig) Validate() error {
	if math.IsNaN(c.Amount) || math.IsInf(c.Amount, 0) || c.Amount < 0 || c.Amount > 1e6 || QuantizeUsageBillingAmount(c.Amount) != c.Amount {
		return errors.New("amount must be between 0 and 1000000 USD with at most 8 decimal places")
	}
	if len(strings.TrimSpace(c.MatchDescription)) == 0 || len(c.MatchDescription) > 4096 {
		return errors.New("match description must contain 1 to 4096 bytes")
	}
	if len(strings.TrimSpace(c.ClientMessage)) == 0 || len(c.ClientMessage) > 4096 {
		return errors.New("client message must contain 1 to 4096 bytes")
	}
	if c.Enabled && c.Amount == 0 {
		return errors.New("enabled billing requires a positive amount")
	}
	return nil
}

func (s *ContentSafetyBillingService) UpdateConfig(ctx context.Context, c ContentSafetyBillingConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE content_safety_config SET enabled=?,amount_units=?,match_description=?,client_message=? WHERE id=1`, c.Enabled, int64(math.Round(c.Amount*1e8)), c.MatchDescription, c.ClientMessage)
	return err
}

// Match only the decoded error description, never a whole JSON envelope whose
// unrelated fields could accidentally trigger a debit. Plain-text errors work too.
func (s *ContentSafetyBillingService) Match(ctx context.Context, status int, body []byte) (*ContentSafetyBillingConfig, error) {
	if s == nil || status != 403 {
		return nil, nil
	}
	c, err := s.Config(ctx)
	if err != nil {
		return nil, err
	}
	if !c.Enabled {
		return nil, nil
	}
	message := extractUpstreamErrorMessage(body)
	if !json.Valid(body) {
		message = string(body)
	} else if message == "" {
		// Native xAI errors also use {"code":"PermissionDenied","error":"..."}.
		if value := gjson.GetBytes(body, "error"); value.Type == gjson.String {
			message = value.String()
		} else {
			var value string
			if json.Unmarshal(body, &value) == nil {
				message = value
			}
		}
	}
	if !strings.Contains(message, c.MatchDescription) {
		return nil, nil
	}
	return &c, nil
}

func (s *ContentSafetyBillingService) Debit(ctx context.Context, d ContentSafetyDebit, c ContentSafetyBillingConfig) error {
	if d.ID == "" || d.UserID <= 0 || d.APIKeyID <= 0 || d.AccountID <= 0 {
		return errors.New("invalid content safety debit identity")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if !c.Enabled {
		return errors.New("content safety billing is disabled")
	}
	// Snapshot all money-affecting values before contacting the primary database.
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO content_safety_debits
	 (id,user_id,api_key_id,account_id,upstream_request_id,amount_units,notes,client_message,created_at)
	 VALUES(?,?,?,?,?,?,?,?,?)`, d.ID, d.UserID, d.APIKeyID, d.AccountID, d.UpstreamRequestID, int64(math.Round(c.Amount*1e8)), c.MatchDescription, c.ClientMessage, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return s.settle(ctx, d.ID)
}

const contentSafetyDebitColumns = `id,user_id,api_key_id,account_id,upstream_request_id,amount_units,notes,client_message,status,created_at,charged_at,last_error`

type contentSafetyScanner interface{ Scan(...any) error }

func scanContentSafetyDebit(row contentSafetyScanner) (ContentSafetyDebit, error) {
	var d ContentSafetyDebit
	var units int64
	err := row.Scan(&d.ID, &d.UserID, &d.APIKeyID, &d.AccountID, &d.UpstreamRequestID, &units, &d.Notes, &d.ClientMessage, &d.Status, &d.CreatedAt, &d.ChargedAt, &d.LastError)
	d.Amount = float64(units) / 1e8
	return d, err
}

func (s *ContentSafetyBillingService) settle(ctx context.Context, id string) error {
	s.processMu.Lock()
	defer s.processMu.Unlock()
	d, err := scanContentSafetyDebit(s.db.QueryRowContext(ctx, `SELECT `+contentSafetyDebitColumns+` FROM content_safety_debits WHERE id=?`, id))
	if err != nil || d.Status == "charged" {
		return err
	}
	if s.billing == nil {
		return errors.New("content safety billing repository unavailable")
	}
	_, err = s.billing.Apply(ctx, &UsageBillingCommand{
		RequestID: contentSafetyUsageRequestID(d.ID), APIKeyID: d.APIKeyID, UserID: d.UserID,
		AccountID: d.AccountID, Model: ContentSafetyUsageModel, BalanceCost: d.Amount,
	})
	if err != nil {
		// The main commit may have succeeded even when the connection failed. Keep
		// pending and replay the SAME key; never issue a second, fresh debit.
		_, recordErr := s.db.ExecContext(ctx, `UPDATE content_safety_debits SET last_error='Balance settlement pending; automatic retry scheduled',attempted_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id)
		return errors.Join(err, recordErr)
	}
	if err := s.recordUsage(ctx, d); err != nil {
		_, recordErr := s.db.ExecContext(ctx, `UPDATE content_safety_debits SET last_error='Usage record pending; automatic retry scheduled',attempted_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id)
		return errors.Join(err, recordErr)
	}
	if s.invalidate != nil {
		if err := s.invalidate(ctx, d.UserID); err != nil {
			return fmt.Errorf("invalidate safety debit balance cache: %w", err)
		}
	}
	_, err = s.db.ExecContext(ctx, `UPDATE content_safety_debits SET status='charged',charged_at=?,last_error='' WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func contentSafetyUsageRequestID(id string) string {
	return "content-safety:" + id
}

func (s *ContentSafetyBillingService) recordUsage(ctx context.Context, d ContentSafetyDebit) error {
	if s.usage == nil {
		return errors.New("content safety usage recorder unavailable")
	}
	mode := ContentSafetyBillingMode
	accountCost := 0.0
	entry := &UsageLog{
		UserID: d.UserID, APIKeyID: d.APIKeyID, AccountID: d.AccountID,
		RequestID: contentSafetyUsageRequestID(d.ID), Model: ContentSafetyUsageModel, RequestedModel: ContentSafetyUsageModel,
		BillingMode: &mode, BillingType: BillingTypeBalance, RequestType: RequestTypeCyberBlocked,
		TotalCost: d.Amount, ActualCost: d.Amount, RateMultiplier: 1, AccountStatsCost: &accountCost,
	}
	if createdAt, err := time.Parse(time.RFC3339Nano, d.CreatedAt); err == nil {
		entry.CreatedAt = createdAt
	}
	if upstream := strings.TrimSpace(d.UpstreamRequestID); upstream != "" && len(upstream) <= 128 {
		entry.UpstreamRequestID = &upstream
	}
	if _, err := s.usage.Create(ctx, entry); err != nil {
		return err
	}
	return nil
}

func (s *ContentSafetyBillingService) Records(ctx context.Context, page, pageSize int) (ContentSafetyDebitPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	out := ContentSafetyDebitPage{Items: []ContentSafetyDebit{}, Page: page, PageSize: pageSize}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM content_safety_debits`).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+contentSafetyDebitColumns+` FROM content_safety_debits ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		d, err := scanContentSafetyDebit(rows)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, d)
	}
	return out, rows.Err()
}

func (s *ContentSafetyBillingService) Recover(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM content_safety_debits WHERE status='pending' ORDER BY attempted_at,created_at LIMIT 100`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	err = errors.Join(err, rows.Close())
	if err != nil {
		return err
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.settle(ctx, id); err != nil {
			slog.Error("content safety debit recovery pending", "debit_id", id, "error", err)
		}
	}
	return nil
}

func (s *ContentSafetyBillingService) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			runCtx, stop := context.WithTimeout(ctx, 20*time.Second)
			if err := s.Recover(runCtx); err != nil && ctx.Err() == nil {
				slog.Error("content safety recovery failed", "error", err)
			}
			stop()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *ContentSafetyBillingService) Close() error {
	if s.cancel != nil {
		s.cancel()
		<-s.done
	}
	return s.db.Close()
}

func newContentSafetyDebitID() string { return uuid.NewString() }
