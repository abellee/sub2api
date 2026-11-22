package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/latencytrace"
)

var _ latencytrace.Repository = (*opsRepository)(nil)

const latencySessionColumns = `sequence,id,user_id,group_id,account_id,created_by,started_at,stopped_at,stop_reason,record_count`

func (r *opsRepository) CreateLatencySession(ctx context.Context, s *latencytrace.Session) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize starts across application instances, including the global limit.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(262001)`); err != nil {
		return err
	}
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND deleted_at IS NULL)
		AND EXISTS(SELECT 1 FROM groups WHERE id=$2 AND deleted_at IS NULL)
		AND EXISTS(SELECT 1 FROM accounts WHERE id=$3 AND deleted_at IS NULL)`, s.UserID, s.GroupID, s.AccountID).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return latencytrace.ErrInvalidScope
	}
	var count int
	var duplicate bool
	err = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(bool_or(user_id=$1 AND group_id=$2 AND account_id=$3),false)
		FROM ops_latency_collections WHERE stopped_at IS NULL`, s.UserID, s.GroupID, s.AccountID).Scan(&count, &duplicate)
	if err != nil {
		return err
	}
	if count >= 10 || duplicate {
		return latencytrace.ErrActiveLimit
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO ops_latency_collections(id,user_id,group_id,account_id,created_by)
		VALUES($1,$2,$3,$4,$5) RETURNING sequence,started_at`, s.ID, s.UserID, s.GroupID, s.AccountID, s.CreatedBy).Scan(&s.Sequence, &s.StartedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *opsRepository) StopLatencySession(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE ops_latency_collections SET stopped_at=COALESCE(stopped_at,clock_timestamp()),
		stop_reason=CASE WHEN stopped_at IS NULL THEN 'manual' ELSE stop_reason END WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return latencytrace.ErrNotFound
	}
	return nil
}

func scanLatencySessions(rows *sql.Rows) ([]latencytrace.Session, error) {
	defer func() { _ = rows.Close() }()
	items := make([]latencytrace.Session, 0)
	for rows.Next() {
		var s latencytrace.Session
		if err := rows.Scan(&s.Sequence, &s.ID, &s.UserID, &s.GroupID, &s.AccountID, &s.CreatedBy, &s.StartedAt, &s.StoppedAt, &s.StopReason, &s.RecordCount); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}
func (r *opsRepository) ListLatencySessions(ctx context.Context, id string, before int64) ([]latencytrace.Session, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+latencySessionColumns+` FROM ops_latency_collections
		WHERE ($1='' OR id::text=$1) AND ($2::bigint=0 OR sequence<$2) ORDER BY sequence DESC LIMIT 50`, id, before)
	if err != nil {
		return nil, err
	}
	return scanLatencySessions(rows)
}
func (r *opsRepository) ActiveLatencySessions(ctx context.Context) ([]latencytrace.Session, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+latencySessionColumns+` FROM ops_latency_collections WHERE stopped_at IS NULL ORDER BY sequence LIMIT 10`)
	if err != nil {
		return nil, err
	}
	return scanLatencySessions(rows)
}

func (r *opsRepository) InsertLatencyRecord(ctx context.Context, record *latencytrace.Record) error {
	detail, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var active bool
	var count int
	// Stop takes the same row lock. A queued/in-flight request cannot append
	// diagnostic records after the administrator's stop transaction commits.
	err = tx.QueryRowContext(ctx, `SELECT stopped_at IS NULL,record_count FROM ops_latency_collections WHERE id=$1 FOR UPDATE`, record.CollectionID).Scan(&active, &count)
	if err != nil {
		return err
	}
	if !active || count >= latencytrace.MaxRecords {
		return nil
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ops_latency_records(collection_id,request_id,client_request_id,detail) VALUES($1,$2,$3,$4)`, record.CollectionID, record.RequestID, record.ClientRequestID, string(detail))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE ops_latency_collections SET record_count=record_count+1,
		stopped_at=CASE WHEN record_count+1 >= 10000 THEN clock_timestamp() ELSE NULL END,
		stop_reason=CASE WHEN record_count+1 >= 10000 THEN 'limit' ELSE '' END WHERE id=$1`, record.CollectionID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *opsRepository) ListLatencyRecords(ctx context.Context, id, requestID string, before int64) ([]latencytrace.Record, error) {
	clientID := strings.TrimPrefix(requestID, "client:")
	rows, err := r.db.QueryContext(ctx, `SELECT id,detail FROM ops_latency_records WHERE collection_id=$1
		AND ($2='' OR request_id=$2 OR client_request_id=$3) AND ($4::bigint=0 OR id<$4) ORDER BY id DESC LIMIT 50`, id, requestID, clientID, before)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]latencytrace.Record, 0)
	for rows.Next() {
		var rowID int64
		var detail []byte
		if err = rows.Scan(&rowID, &detail); err != nil {
			return nil, err
		}
		var record latencytrace.Record
		if err = json.Unmarshal(detail, &record); err != nil {
			return nil, err
		}
		record.ID = rowID
		items = append(items, record)
	}
	return items, rows.Err()
}
