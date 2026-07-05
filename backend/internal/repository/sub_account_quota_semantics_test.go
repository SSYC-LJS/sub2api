package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSubAccountUpdateQuotaTreatsPermanentQuotaAsCurrentAvailable(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	repo := &subAccountRepository{db: db}
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta("allocated_quota=used_quota + $1")).
		WithArgs(12.5, 3.5, int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, parent_user_id, child_user_id, allocated_quota, used_quota, weekly_allocated_quota")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_user_id", "child_user_id", "allocated_quota", "used_quota", "weekly_allocated_quota", "weekly_used_quota", "weekly_window_start", "status", "created_at", "updated_at", "deleted_at"}).
			AddRow(int64(9), int64(1), int64(2), 20.5, 8.0, 3.5, 0.5, now, "active", now, now, nil))

	rel, err := repo.UpdateQuota(context.Background(), 1, 2, 12.5, 3.5)
	require.NoError(t, err)
	require.Equal(t, 20.5, rel.AllocatedQuota)
	require.Equal(t, 12.5, rel.RemainingQuota())
	require.NoError(t, mock.ExpectationsWereMet())
}
