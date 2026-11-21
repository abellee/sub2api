package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *groupRepository) ObserveSyncIsolation(ctx context.Context, keyID, userID, groupID int64, stream bool, observedAt time.Time) (int64, bool, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	current, err := client.Group.Query().Where(group.IDEQ(groupID)).Only(ctx)
	if err != nil {
		return 0, false, err
	}
	sourceID := current.ID
	if current.SyncIsolationSourceGroupID > 0 {
		sourceID = current.SyncIsolationSourceGroupID
	}
	source, err := client.Group.Query().Where(group.IDEQ(sourceID)).ForShare().Only(ctx)
	if err != nil {
		return 0, false, err
	}
	// A per-user/source row serializes requests from different keys and replicas.
	if _, err = client.ExecContext(ctx, `INSERT INTO sync_isolation_windows (user_id, group_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, sourceID); err != nil {
		return 0, false, err
	}
	var raw []byte
	if err = scanSingleRow(ctx, client, `SELECT state FROM sync_isolation_windows WHERE user_id=$1 AND group_id=$2 FOR UPDATE`, []any{userID, sourceID}, &raw); err != nil {
		return 0, false, err
	}
	key, err := client.APIKey.Query().Where(apikey.IDEQ(keyID), apikey.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		return 0, false, err
	}
	if key.GroupID == nil {
		return 0, false, tx.Commit()
	}
	actual, err := client.Group.Get(ctx, *key.GroupID)
	if err != nil {
		return 0, false, err
	}
	if actual.ID != sourceID && actual.SyncIsolationSourceGroupID != sourceID {
		// An administrator moved this key to an unrelated group while auth was cached.
		return actual.ID, false, tx.Commit()
	}
	// Permissions may have changed after the request queued this observation.
	var allowed bool
	if err = scanSingleRow(ctx, client, `SELECT
		(g.subscription_type='subscription' OR (NOT g.is_exclusive AND NOT u.restrict_public_groups)
		 OR EXISTS (SELECT 1 FROM user_allowed_groups WHERE user_id=u.id AND group_id=g.id FOR SHARE))
		AND ($3=g.id OR EXISTS (SELECT 1 FROM user_allowed_groups WHERE user_id=u.id AND group_id=$3 FOR SHARE))
		FROM users u JOIN groups g ON g.id=$2
		WHERE u.id=$1 AND u.deleted_at IS NULL AND u.status='active'
		AND g.deleted_at IS NULL AND g.status='active' FOR SHARE OF u`, []any{userID, sourceID, actual.ID}, &allowed); err != nil {
		return 0, false, err
	}
	if !allowed {
		return actual.ID, false, tx.Commit()
	}
	var window service.SyncIsolationWindow
	if err = json.Unmarshal(raw, &window); err != nil {
		return 0, false, err
	}
	window.Observe(observedAt, stream, source.SyncIsolationEnabled)
	targetID := sourceID
	if window.Isolated {
		child, findErr := client.Group.Query().Where(group.SyncIsolationSourceGroupIDEQ(sourceID)).Only(ctx)
		if findErr != nil && !dbent.IsNotFound(findErr) {
			return 0, false, findErr
		}
		if child == nil {
			// Different users may cross the threshold together; serialize creation.
			if _, err = client.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, -sourceID); err != nil {
				return 0, false, err
			}
			child, findErr = client.Group.Query().Where(group.SyncIsolationSourceGroupIDEQ(sourceID)).Only(ctx)
			if findErr != nil && !dbent.IsNotFound(findErr) {
				return 0, false, findErr
			}
		}
		if child == nil {
			copy := *groupEntityToService(source)
			copy.ID = 0
			copy.DuplicateOperationID = ""
			copy.SyncIsolationSourceGroupID = sourceID
			copy.SyncIsolationEnabled = false
			copy.IsExclusive = true
			copy.Status = service.StatusActive
			// Include the source ID so truncated names cannot collide across sources.
			suffix := fmt.Sprintf(" [同步-%d]", sourceID)
			name := []rune(source.Name)
			if len(name)+len([]rune(suffix)) > 100 {
				name = name[:100-len([]rune(suffix))]
			}
			copy.Name = string(name) + suffix
			copy.I18n = make(service.GroupI18n, len(source.I18n))
			for locale, text := range source.I18n {
				text.Name = ""
				copy.I18n[locale] = text
			}
			if err = createGroupRecord(ctx, client, &copy); err != nil {
				return 0, false, err
			}
			targetID = copy.ID
			if _, err = client.ExecContext(ctx, `SELECT sync_isolation_bindings($1,$2)`, sourceID, targetID); err != nil {
				return 0, false, err
			}
			if err = enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventGroupChanged, nil, &targetID, nil); err != nil {
				return 0, false, err
			}
		} else {
			targetID = child.ID
			if child.Status != service.StatusActive {
				if err = client.Group.UpdateOneID(child.ID).SetStatus(service.StatusActive).Exec(ctx); err != nil {
					return 0, false, err
				}
			}
		}
		if _, err = client.ExecContext(ctx, `INSERT INTO user_allowed_groups (user_id,group_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, targetID); err != nil {
			return 0, false, err
		}
	}
	// Move this user's corresponding keys only; preserve all other key fields.
	result, err := client.ExecContext(ctx, `UPDATE api_keys SET group_id=$3, updated_at=NOW()
		WHERE user_id=$1 AND deleted_at IS NULL AND group_id<>$3
		AND (group_id=$2 OR group_id IN (SELECT id FROM groups WHERE sync_isolation_source_group_id=$2))`, userID, sourceID, targetID)
	if err != nil {
		return 0, false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	raw, err = json.Marshal(window)
	if err != nil {
		return 0, false, err
	}
	if _, err = client.ExecContext(ctx, `UPDATE sync_isolation_windows SET state=$3, updated_at=NOW() WHERE user_id=$1 AND group_id=$2`, userID, sourceID, raw); err != nil {
		return 0, false, err
	}
	return targetID, count > 0, tx.Commit()
}
