package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type subAccountRepository struct {
	db *sql.DB
}

func NewSubAccountRepository(sqlDB *sql.DB) service.SubAccountRepository {
	return &subAccountRepository{db: sqlDB}
}

func (r *subAccountRepository) ListByParent(ctx context.Context, parentUserID int64, params pagination.PaginationParams) ([]service.SubAccountRelation, *pagination.PaginationResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	offset := (params.Page - 1) * params.PageSize
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM parent_child_accounts WHERE parent_user_id=$1 AND status='active' AND deleted_at IS NULL`, parentUserID).Scan(&total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT pca.id, pca.parent_user_id, pca.child_user_id, pca.allocated_quota, pca.used_quota, pca.status, pca.created_at, pca.updated_at, pca.deleted_at,
		       u.email, u.username, u.role, u.balance, u.concurrency, u.status, u.rpm_limit, u.is_parent_account, u.created_at, u.updated_at, u.deleted_at
		FROM parent_child_accounts pca
		JOIN users u ON u.id = pca.child_user_id AND u.deleted_at IS NULL
		WHERE pca.parent_user_id=$1 AND pca.status='active' AND pca.deleted_at IS NULL
		ORDER BY pca.created_at DESC, pca.id DESC
		LIMIT $2 OFFSET $3
	`, parentUserID, params.PageSize, offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]service.SubAccountRelation, 0)
	for rows.Next() {
		var rel service.SubAccountRelation
		var deletedAt sql.NullTime
		var child service.User
		var childDeletedAt sql.NullTime
		if err := rows.Scan(&rel.ID, &rel.ParentUserID, &rel.ChildUserID, &rel.AllocatedQuota, &rel.UsedQuota, &rel.Status, &rel.CreatedAt, &rel.UpdatedAt, &deletedAt,
			&child.Email, &child.Username, &child.Role, &child.Balance, &child.Concurrency, &child.Status, &child.RPMLimit, &child.IsParentAccount, &child.CreatedAt, &child.UpdatedAt, &childDeletedAt); err != nil {
			return nil, nil, err
		}
		child.ID = rel.ChildUserID
		if deletedAt.Valid {
			rel.DeletedAt = &deletedAt.Time
		}
		if childDeletedAt.Valid {
			child.DeletedAt = &childDeletedAt.Time
		}
		rel.ChildUser = &child
		items = append(items, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.PageSize, Pages: int((total + int64(params.PageSize) - 1) / int64(params.PageSize))}, nil
}

func (r *subAccountRepository) GetActiveByParentAndChild(ctx context.Context, parentUserID, childUserID int64) (*service.SubAccountRelation, error) {
	return r.scanOne(ctx, `SELECT id, parent_user_id, child_user_id, allocated_quota, used_quota, status, created_at, updated_at, deleted_at FROM parent_child_accounts WHERE parent_user_id=$1 AND child_user_id=$2 AND status='active' AND deleted_at IS NULL`, parentUserID, childUserID)
}

func (r *subAccountRepository) GetActiveByChild(ctx context.Context, childUserID int64) (*service.SubAccountRelation, error) {
	return r.scanOne(ctx, `SELECT id, parent_user_id, child_user_id, allocated_quota, used_quota, status, created_at, updated_at, deleted_at FROM parent_child_accounts WHERE child_user_id=$1 AND status='active' AND deleted_at IS NULL`, childUserID)
}

func (r *subAccountRepository) scanOne(ctx context.Context, query string, args ...any) (*service.SubAccountRelation, error) {
	var rel service.SubAccountRelation
	var deletedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&rel.ID, &rel.ParentUserID, &rel.ChildUserID, &rel.AllocatedQuota, &rel.UsedQuota, &rel.Status, &rel.CreatedAt, &rel.UpdatedAt, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSubAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		rel.DeletedAt = &deletedAt.Time
	}
	return &rel, nil
}

func (r *subAccountRepository) Upsert(ctx context.Context, parentUserID int64, input service.SubAccountUpsertInput) (*service.SubAccountRelation, error) {
	if parentUserID == input.ChildUserID {
		return nil, service.ErrSubAccountSelfLink
	}
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO parent_child_accounts(parent_user_id, child_user_id, allocated_quota, used_quota, status)
		VALUES($1,$2,$3,0,'active')
		RETURNING id
	`, parentUserID, input.ChildUserID, input.AllocatedQuota).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "chk_parent_child_accounts_different_users") {
			return nil, service.ErrSubAccountSelfLink
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, service.ErrSubAccountAlreadyLinked
		}
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *subAccountRepository) GetByID(ctx context.Context, id int64) (*service.SubAccountRelation, error) {
	return r.scanOne(ctx, `SELECT id, parent_user_id, child_user_id, allocated_quota, used_quota, status, created_at, updated_at, deleted_at FROM parent_child_accounts WHERE id=$1 AND deleted_at IS NULL`, id)
}

func (r *subAccountRepository) UpdateQuota(ctx context.Context, parentUserID, childUserID int64, allocatedQuota float64) (*service.SubAccountRelation, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `UPDATE parent_child_accounts SET allocated_quota=$1, updated_at=NOW() WHERE parent_user_id=$2 AND child_user_id=$3 AND status='active' AND deleted_at IS NULL RETURNING id`, allocatedQuota, parentUserID, childUserID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSubAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *subAccountRepository) Remove(ctx context.Context, parentUserID, childUserID int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE parent_child_accounts SET status='disabled', deleted_at=NOW(), updated_at=NOW() WHERE parent_user_id=$1 AND child_user_id=$2 AND status='active' AND deleted_at IS NULL`, parentUserID, childUserID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrSubAccountNotFound
	}
	return nil
}

func (r *subAccountRepository) IncrementUsageIfAvailable(ctx context.Context, tx *sql.Tx, childUserID int64, amount float64) (*service.SubAccountRelation, error) {
	if amount <= 0 {
		return nil, nil
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE parent_child_accounts
		SET used_quota = used_quota + $1, updated_at = NOW()
		WHERE id = (
			SELECT id FROM parent_child_accounts
			WHERE child_user_id=$2 AND status='active' AND deleted_at IS NULL AND allocated_quota - used_quota >= $1
			ORDER BY id DESC LIMIT 1 FOR UPDATE
		)
		RETURNING id, parent_user_id, child_user_id, allocated_quota, used_quota, status, created_at, updated_at, deleted_at
	`, amount, childUserID)
	var rel service.SubAccountRelation
	var deletedAt sql.NullTime
	err := row.Scan(&rel.ID, &rel.ParentUserID, &rel.ChildUserID, &rel.AllocatedQuota, &rel.UsedQuota, &rel.Status, &rel.CreatedAt, &rel.UpdatedAt, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("increment parent quota: %w", err)
	}
	if deletedAt.Valid {
		rel.DeletedAt = &deletedAt.Time
	}
	return &rel, nil
}
