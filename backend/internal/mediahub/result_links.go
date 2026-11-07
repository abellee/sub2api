package mediahub

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var safeTaskID = regexp.MustCompile(`^mtask_[a-zA-Z0-9_-]+$`)
var safeResultID = regexp.MustCompile(`^f[0-9]{4,8}$`)

type ResultLink struct {
	ID          string    `json:"id"`
	UserID      int64     `json:"user_id"`
	TaskID      string    `json:"task_id"`
	FileID      string    `json:"file_id"`
	Model       string    `json:"model"`
	DownloadURL string    `json:"download_url"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"created_at"`
}

// ResultLinkStore stores metadata only. Bytes remain on the media service.
type ResultLinkStore struct{ db *sql.DB }

func NewResultLinkStore(db *sql.DB) *ResultLinkStore { return &ResultLinkStore{db: db} }

func (s *ResultLinkStore) Prepare(ctx context.Context, req SettleRequest, base string) error {
	if s == nil || s.db == nil {
		return errors.New("media result database is not configured")
	}
	if req.UserID <= 0 || req.APIKeyID <= 0 || !safeTaskID.MatchString(req.TaskID) || len(req.Files) == 0 {
		return errors.New("invalid media result owner or task")
	}
	root, err := url.Parse(strings.TrimRight(base, "/") + "/")
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, file := range req.Files {
		sum, err := hex.DecodeString(file.SHA256)
		if err != nil || len(sum) != sha256.Size || file.Size <= 0 || !safeResultID.MatchString(file.ID) {
			return errors.New("invalid media result metadata")
		}
		ref, err := url.Parse(file.DownloadURL)
		if err != nil || file.DownloadURL == "" {
			return errors.New("missing media download URL")
		}
		link := root.ResolveReference(ref)
		if (link.Scheme != "http" && link.Scheme != "https") || link.Host == "" || link.User != nil || link.Fragment != "" {
			return errors.New("invalid media download URL")
		}
		digest := sha256.Sum256([]byte(req.TaskID + "/" + file.ID))
		id := "mf" + hex.EncodeToString(digest[:16])
		_, err = tx.ExecContext(ctx, `INSERT INTO media_generation_files (id,user_id,api_key_id,task_id,file_id,model,download_url,content_type,size_bytes,sha256) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (task_id,file_id) DO NOTHING`, id, req.UserID, req.APIKeyID, req.TaskID, file.ID, req.Model, link.String(), file.ContentType, file.Size, file.SHA256)
		if err != nil {
			return err
		}
		var owner, keyID, size int64
		var checksum string
		if err = tx.QueryRowContext(ctx, `SELECT user_id,api_key_id,size_bytes,sha256 FROM media_generation_files WHERE task_id=$1 AND file_id=$2`, req.TaskID, file.ID).Scan(&owner, &keyID, &size, &checksum); err != nil {
			return err
		}
		if owner != req.UserID || keyID != req.APIKeyID || size != file.Size || checksum != file.SHA256 {
			return fmt.Errorf("media result conflicts with existing owner or metadata")
		}
	}
	return tx.Commit()
}
func (s *ResultLinkStore) MarkReady(ctx context.Context, userID int64, taskID string) error {
	if s == nil || s.db == nil {
		return errors.New("media result database is not configured")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE media_generation_files SET ready=true WHERE user_id=$1 AND task_id=$2`, userID, taskID)
	return err
}
func (s *ResultLinkStore) List(ctx context.Context, userID int64, taskID string) ([]ResultLink, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("media result database is not configured")
	}
	query := `SELECT id,user_id,task_id,file_id,model,download_url,content_type,size_bytes,sha256,created_at FROM media_generation_files WHERE user_id=$1 AND ready=true AND ($2='' OR task_id=$2) ORDER BY created_at DESC,id`
	if taskID == "" {
		query += " LIMIT 200"
	}
	rows, err := s.db.QueryContext(ctx, query, userID, taskID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []ResultLink{}
	for rows.Next() {
		var v ResultLink
		if err = rows.Scan(&v.ID, &v.UserID, &v.TaskID, &v.FileID, &v.Model, &v.DownloadURL, &v.ContentType, &v.SizeBytes, &v.SHA256, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *ResultLinkStore) Get(ctx context.Context, userID int64, id string) (*ResultLink, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("media result database is not configured")
	}
	var v ResultLink
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,task_id,file_id,model,download_url,content_type,size_bytes,sha256,created_at FROM media_generation_files WHERE user_id=$1 AND id=$2 AND ready=true`, userID, id).Scan(&v.ID, &v.UserID, &v.TaskID, &v.FileID, &v.Model, &v.DownloadURL, &v.ContentType, &v.SizeBytes, &v.SHA256, &v.CreatedAt)
	return &v, err
}
