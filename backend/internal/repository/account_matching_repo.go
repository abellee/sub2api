package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type accountMatchingRepository struct{ db *sql.DB }

func NewAccountMatchingRepository(db *sql.DB) service.AccountMatchingRepository {
	return &accountMatchingRepository{db: db}
}

func (r *accountMatchingRepository) List(ctx context.Context) ([]service.AccountMatchingRule, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.id, r.user_id, u.email, r.group_ids, r.account_ids, r.created_at, r.updated_at
        FROM account_matching_rules r JOIN users u ON u.id = r.user_id WHERE u.deleted_at IS NULL ORDER BY r.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]service.AccountMatchingRule, 0)
	for rows.Next() {
		var rule service.AccountMatchingRule
		if err := rows.Scan(&rule.ID, &rule.UserID, &rule.Email, pq.Array(&rule.GroupIDs), pq.Array(&rule.AccountIDs), &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func validMatchingIDs(ids []int64) bool {
	if len(ids) == 0 || len(ids) > 10000 {
		return false
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (r *accountMatchingRepository) Save(ctx context.Context, rule *service.AccountMatchingRule) error {
	if rule.UserID <= 0 || !validMatchingIDs(rule.GroupIDs) || !validMatchingIDs(rule.AccountIDs) {
		return service.ErrAccountMatchingInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialise edits for this user so concurrent overlapping rules cannot be saved.
	if err := tx.QueryRowContext(ctx, `SELECT email FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, rule.UserID).Scan(&rule.Email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrAccountMatchingInvalid
		}
		return err
	}
	var valid bool
	if err := tx.QueryRowContext(ctx, `SELECT
        (SELECT count(*) FROM groups WHERE id = ANY($1) AND deleted_at IS NULL) = cardinality($1::bigint[])
        AND (SELECT count(*) FROM accounts WHERE id = ANY($2) AND deleted_at IS NULL) = cardinality($2::bigint[])`, pq.Array(rule.GroupIDs), pq.Array(rule.AccountIDs)).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return service.ErrAccountMatchingInvalid
	}
	var overlaps bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM account_matching_rules WHERE user_id = $1 AND id <> $2 AND group_ids && $3::bigint[])`, rule.UserID, rule.ID, pq.Array(rule.GroupIDs)).Scan(&overlaps); err != nil {
		return err
	}
	if overlaps {
		return service.ErrAccountMatchingConflict
	}
	if rule.ID == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO account_matching_rules(user_id, group_ids, account_ids) VALUES($1,$2,$3) RETURNING id, created_at, updated_at`, rule.UserID, pq.Array(rule.GroupIDs), pq.Array(rule.AccountIDs)).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	} else {
		// Keep the owner fixed; changing the owner is a new rule.
		err = tx.QueryRowContext(ctx, `UPDATE account_matching_rules SET group_ids=$3, account_ids=$4, updated_at=NOW() WHERE id=$1 AND user_id=$2 RETURNING created_at, updated_at`, rule.ID, rule.UserID, pq.Array(rule.GroupIDs), pq.Array(rule.AccountIDs)).Scan(&rule.CreatedAt, &rule.UpdatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrAccountMatchingNotFound
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *accountMatchingRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM account_matching_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return service.ErrAccountMatchingNotFound
	}
	return err
}

func (r *accountMatchingRepository) AllowedAccounts(ctx context.Context, userID, groupID int64) ([]int64, error) {
	var ids []int64
	err := r.db.QueryRowContext(ctx, `SELECT account_ids FROM account_matching_rules WHERE user_id=$1 AND $2=ANY(group_ids)`, userID, groupID).Scan(pq.Array(&ids))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return ids, err
}

func (r *accountMatchingRepository) SearchUsers(ctx context.Context, email string) ([]service.AccountMatchingUser, error) {
	users := make([]service.AccountMatchingUser, 0)
	email = strings.TrimSpace(email)
	if email == "" {
		return users, nil
	}
	email = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(email)
	rows, err := r.db.QueryContext(ctx, `SELECT id, email FROM users WHERE deleted_at IS NULL AND email ILIKE $1 ORDER BY email LIMIT 30`, "%"+email+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u service.AccountMatchingUser
		if err := rows.Scan(&u.ID, &u.Email); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
