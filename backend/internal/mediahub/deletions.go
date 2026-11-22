package mediahub

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// FileDeletionEvent reports physical removal, independently of generation billing.
type FileDeletionEvent struct {
	Event     string    `json:"event"`
	EventID   string    `json:"event_id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	KeyID     string    `json:"key_id"`
	FileIDs   []string  `json:"file_ids"`
	DeletedAt time.Time `json:"deleted_at"`
	Reason    string    `json:"reason"`
}

type FileDeletionSink interface {
	MarkDeleted(context.Context, FileDeletionEvent) error
}

func (s *ResultLinkStore) MarkDeleted(ctx context.Context, ev FileDeletionEvent) error {
	if s == nil || s.db == nil {
		return errors.New("media result database is not configured")
	}
	userID, userErr := strconv.ParseInt(ev.UserID, 10, 64)
	keyID, keyErr := strconv.ParseInt(ev.KeyID, 10, 64)
	if ev.Event != "files.deleted" || len(ev.EventID) == 0 || len(ev.EventID) > 128 || !safeTaskID.MatchString(ev.TaskID) || userErr != nil || keyErr != nil || userID <= 0 || keyID <= 0 || len(ev.FileIDs) == 0 || len(ev.FileIDs) > 256 || ev.DeletedAt.Unix() <= 0 || ev.DeletedAt.After(time.Now().Add(5*time.Minute)) || (ev.Reason != "retention_expired" && ev.Reason != "files_missing") {
		return errors.New("invalid file deletion event")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, fileID := range ev.FileIDs {
		if !safeResultID.MatchString(fileID) {
			return errors.New("invalid deleted file ID")
		}
		var owner, key int64
		if err := tx.QueryRowContext(ctx, `SELECT user_id,COALESCE(api_key_id,0) FROM media_generation_files WHERE task_id=$1 AND file_id=$2`, ev.TaskID, fileID).Scan(&owner, &key); err != nil {
			return err
		}
		if owner != userID || key != keyID {
			return fmt.Errorf("media deletion owner mismatch")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE media_generation_files SET delete_reason=CASE WHEN deleted_at IS NULL THEN $4 ELSE delete_reason END,deleted_at=COALESCE(deleted_at,$3) WHERE task_id=$1 AND file_id=$2`, ev.TaskID, fileID, ev.DeletedAt.UTC(), ev.Reason); err != nil {
			return err
		}
	}
	return tx.Commit()
}
