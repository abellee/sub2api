package mediahub

import (
	"context"
	"errors"
	"time"
)

// LibraryPage uses a stable cursor so clearing a large library never stops at the legacy 200-row limit.
func (s *ResultLinkStore) LibraryPage(ctx context.Context, userID int64, after, kind string) ([]ResultLink, string, error) {
	if s == nil || s.db == nil {
		return nil, "", errors.New("media result database is not configured")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,task_id,file_id,model,download_url,content_type,size_bytes,sha256,created_at FROM media_generation_files WHERE user_id=$1 AND ready=true AND deleted_at IS NULL AND ($2='' OR (created_at,id)<(SELECT created_at,id FROM media_generation_files WHERE user_id=$1 AND id=$2)) AND (content_type LIKE 'image/%' OR content_type LIKE 'video/%') AND ($3='' OR content_type LIKE $3 || '/%') ORDER BY created_at DESC,id DESC LIMIT 51`, userID, after, kind)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rows.Close() }()
	out := []ResultLink{}
	for rows.Next() {
		var v ResultLink
		if err = rows.Scan(&v.ID, &v.UserID, &v.TaskID, &v.FileID, &v.Model, &v.DownloadURL, &v.ContentType, &v.SizeBytes, &v.SHA256, &v.CreatedAt); err != nil {
			return nil, "", err
		}
		v.applyDeletion()
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > 50 {
		out = out[:50]
		next = out[49].ID
	}
	return out, next, nil
}

func (s *ResultLinkStore) MarkUserDeleted(ctx context.Context, userID int64, id string) error {
	if s == nil || s.db == nil {
		return errors.New("media result database is not configured")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE media_generation_files SET deleted_at=$3,delete_reason='user_deleted' WHERE user_id=$1 AND id=$2 AND deleted_at IS NULL`, userID, id, time.Now())
	return err
}
