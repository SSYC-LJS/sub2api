package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type requestResponseLogRepository struct {
	db *sql.DB
}

func NewRequestResponseLogRepository(db *sql.DB) service.RequestResponseLogRepository {
	return &requestResponseLogRepository{db: db}
}

func (r *requestResponseLogRepository) Create(ctx context.Context, log *service.RequestResponseLog) error {
	if r == nil || r.db == nil || log == nil {
		return nil
	}
	const q = `
		INSERT INTO request_response_logs (
			request_id, user_id, api_key_id, group_id, method, path, endpoint, model, stream,
			status_code, request_body, response_body, request_truncated, response_truncated,
			request_body_bytes, response_body_bytes, duration_ms, user_agent, ip_address, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20
		)`
	if _, err := r.db.ExecContext(ctx, q,
		nullableString(log.RequestID), log.UserID, log.APIKeyID, log.GroupID,
		log.Method, log.Path, nullableString(log.Endpoint), nullableString(log.Model), log.Stream,
		log.StatusCode, log.RequestBody, log.ResponseBody, log.RequestTruncated, log.ResponseTruncated,
		log.RequestBodyBytes, log.ResponseBodyBytes, log.DurationMs, nullableString(log.UserAgent), nullableString(log.IPAddress), log.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert request response log: %w", err)
	}
	return nil
}

func (r *requestResponseLogRepository) List(ctx context.Context, page, pageSize int, filters service.RequestResponseLogFilters) ([]service.RequestResponseLog, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	where, args := buildRequestResponseLogWhere(filters)
	countQuery := "SELECT COUNT(*) FROM request_response_logs" + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count request response logs: %w", err)
	}
	args = append(args, pageSize, (page-1)*pageSize)
	// List query intentionally omits request_body and response_body columns
	// to avoid transferring large payloads for table display.
	// Use GetByID to fetch full body content for detail view.
	query := `
		SELECT id, request_id, user_id, api_key_id, group_id, method, path, endpoint, model, stream,
			status_code, '' AS request_body, '' AS response_body, request_truncated, response_truncated,
			request_body_bytes, response_body_bytes, duration_ms, user_agent, ip_address, created_at
		FROM request_response_logs` + where + `
		ORDER BY created_at DESC, id DESC
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list request response logs: %w", err)
	}
	defer rows.Close()
	items, err := scanRequestResponseLogRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *requestResponseLogRepository) GetByID(ctx context.Context, id int64) (*service.RequestResponseLog, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrSettingNotFound
	}
	const q = `
		SELECT id, request_id, user_id, api_key_id, group_id, method, path, endpoint, model, stream,
			status_code, request_body, response_body, request_truncated, response_truncated,
			request_body_bytes, response_body_bytes, duration_ms, user_agent, ip_address, created_at
		FROM request_response_logs
		WHERE id = $1`
	rows, err := r.db.QueryContext(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("get request response log: %w", err)
	}
	defer rows.Close()
	items, err := scanRequestResponseLogRows(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, service.ErrSettingNotFound
	}
	return &items[0], nil
}

func (r *requestResponseLogRepository) ListForExport(ctx context.Context, filters service.RequestResponseLogFilters, limit int) ([]service.RequestResponseLog, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
	where, args := buildRequestResponseLogWhere(filters)
	args = append(args, limit)
	query := `
		SELECT id, request_id, user_id, api_key_id, group_id, method, path, endpoint, model, stream,
			status_code, request_body, response_body, request_truncated, response_truncated,
			request_body_bytes, response_body_bytes, duration_ms, user_agent, ip_address, created_at
		FROM request_response_logs` + where + `
		ORDER BY created_at DESC, id DESC
		LIMIT $` + fmt.Sprint(len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list request response logs for export: %w", err)
	}
	defer rows.Close()
	return scanRequestResponseLogRows(rows)
}

func buildRequestResponseLogWhere(filters service.RequestResponseLogFilters) (string, []any) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filters.UserID > 0 {
		add("user_id = $%d", filters.UserID)
	}
	if filters.APIKeyID > 0 {
		add("api_key_id = $%d", filters.APIKeyID)
	}
	if filters.GroupID > 0 {
		add("group_id = $%d", filters.GroupID)
	}
	if strings.TrimSpace(filters.Endpoint) != "" {
		add("endpoint = $%d", strings.TrimSpace(filters.Endpoint))
	}
	if strings.TrimSpace(filters.Model) != "" {
		add("model ILIKE $%d", "%"+strings.TrimSpace(filters.Model)+"%")
	}
	if strings.TrimSpace(filters.Path) != "" {
		add("path ILIKE $%d", "%"+strings.TrimSpace(filters.Path)+"%")
	}
	if strings.TrimSpace(filters.Search) != "" {
		args = append(args, "%"+strings.TrimSpace(filters.Search)+"%")
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf("(request_body ILIKE $%d OR response_body ILIKE $%d OR request_id ILIKE $%d)", idx, idx, idx))
	}
	if filters.StartTime != nil {
		add("created_at >= $%d", *filters.StartTime)
	}
	if filters.EndTime != nil {
		add("created_at < $%d", *filters.EndTime)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func scanRequestResponseLogRows(rows *sql.Rows) ([]service.RequestResponseLog, error) {
	items := make([]service.RequestResponseLog, 0)
	for rows.Next() {
		var item service.RequestResponseLog
		var requestID, endpoint, model, userAgent, ipAddress sql.NullString
		var groupID sql.NullInt64
		if err := rows.Scan(
			&item.ID, &requestID, &item.UserID, &item.APIKeyID, &groupID, &item.Method, &item.Path, &endpoint, &model, &item.Stream,
			&item.StatusCode, &item.RequestBody, &item.ResponseBody, &item.RequestTruncated, &item.ResponseTruncated,
			&item.RequestBodyBytes, &item.ResponseBodyBytes, &item.DurationMs, &userAgent, &ipAddress, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan request response log: %w", err)
		}
		item.RequestID = requestID.String
		if groupID.Valid {
			v := groupID.Int64
			item.GroupID = &v
		}
		item.Endpoint = endpoint.String
		item.Model = model.String
		item.UserAgent = userAgent.String
		item.IPAddress = ipAddress.String
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate request response logs: %w", err)
	}
	return items, nil
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
