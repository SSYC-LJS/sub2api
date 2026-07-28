package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

const upstreamOnlyErrorQueryPattern = `(?s)FROM ops_error_logs\s+WHERE.*AND error_source = 'upstream_http'`

func TestComputeAvailabilityHoursUsesTwelveHourWindow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &channelMonitorRepository{db: db}
	queryPattern := regexp.QuoteMeta(`
		SELECT model,
		       COUNT(*) AS total,
		       COUNT(*) FILTER (WHERE status IN ('operational','degraded')) AS ok,
		       CASE WHEN COUNT(latency_ms) > 0
		            THEN SUM(latency_ms) FILTER (WHERE latency_ms IS NOT NULL)::float8 / COUNT(latency_ms)
		            ELSE NULL END AS avg_latency_ms
		FROM channel_monitor_histories
		WHERE monitor_id = $1
		  AND checked_at >= NOW() - ($2::int || ' hours')::interval
		GROUP BY model
	`)
	mock.ExpectQuery(queryPattern).
		WithArgs(int64(42), 12).
		WillReturnRows(sqlmock.NewRows([]string{"model", "total", "ok", "avg_latency_ms"}).
			AddRow("gpt-test", 8, 6, 175.5))

	got, err := repo.ComputeAvailabilityHours(context.Background(), 42, 12)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, got, 1)
	require.Equal(t, 12, got[0].WindowHours)
	require.Equal(t, 75.0, got[0].AvailabilityPct)
	require.NotNil(t, got[0].AvgLatencyMs)
	require.Equal(t, 175, *got[0].AvgLatencyMs)
}

func TestComputeAvailabilityHoursDefaultsToTwelveHoursAndReturnsNoSamples(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &channelMonitorRepository{db: db}
	mock.ExpectQuery(`(?s)FROM channel_monitor_histories.*' hours'`).
		WithArgs(int64(42), 12).
		WillReturnRows(sqlmock.NewRows([]string{"model", "total", "ok", "avg_latency_ms"}))

	got, err := repo.ComputeAvailabilityHours(context.Background(), 42, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Empty(t, got)
}

func TestListRealUsageGroupMonitorStatsCountsOnlyUpstreamErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &channelMonitorRepository{db: db}
	statsRows := sqlmock.NewRows([]string{
		"group_id", "model", "status", "latency_ms", "availability_12h",
		"req_1h", "ok_1h", "err_1h",
		"req_12h", "ok_12h", "err_12h",
		"req_24h", "ok_24h", "err_24h",
	}).AddRow(
		int64(42), "gpt-test", "failed", int64(250), 75.0,
		4, 3, 1,
		8, 6, 2,
		16, 12, 4,
	)
	mock.ExpectQuery(upstreamOnlyErrorQueryPattern).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(statsRows)

	checkedAt := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	timelineRows := sqlmock.NewRows([]string{"group_id", "status", "latency_ms", "created_at"}).
		AddRow(int64(42), "failed", int64(250), checkedAt)
	mock.ExpectQuery(upstreamOnlyErrorQueryPattern).
		WithArgs(sqlmock.AnyArg(), timelineLimitMax).
		WillReturnRows(timelineRows)

	got, err := repo.ListRealUsageGroupMonitorStats(context.Background(), []int64{42})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	stat := got[42]
	require.NotNil(t, stat)
	require.Equal(t, 75.0, stat.Availability12h)
	require.Equal(t, 8, stat.WindowStats.Requests12h)
	require.Equal(t, 6, stat.WindowStats.Success12h)
	require.Equal(t, 2, stat.WindowStats.Errors12h)
	require.Len(t, stat.Timeline, 1)
	require.Equal(t, "failed", stat.Timeline[0].Status)
}

func TestGetRealUsageGroupMonitorDetailCountsOnlyUpstreamErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &channelMonitorRepository{db: db}
	detailRows := sqlmock.NewRows([]string{
		"model", "status", "latency_ms", "availability_12h", "avg_latency_12h",
	}).AddRow("gpt-test", "operational", int64(180), 90.0, 175.5)
	mock.ExpectQuery(upstreamOnlyErrorQueryPattern).
		WithArgs(int64(42)).
		WillReturnRows(detailRows)

	got, err := repo.GetRealUsageGroupMonitorDetail(context.Background(), 42)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, got.Models, 1)
	require.Equal(t, 90.0, got.Models[0].Availability12h)
	require.Equal(t, "operational", got.Models[0].LatestStatus)
}
