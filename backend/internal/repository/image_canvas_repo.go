package repository

import (
	"context"
	"database/sql"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type imageCanvasHistoryRepository struct {
	db *sql.DB
}

func NewImageCanvasHistoryRepository(db *sql.DB) service.ImageCanvasHistoryRepository {
	return &imageCanvasHistoryRepository{db: db}
}

func (r *imageCanvasHistoryRepository) Create(ctx context.Context, item *service.ImageCanvasHistory) error {
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	return r.db.QueryRowContext(ctx, `
		INSERT INTO image_canvas_histories (
			user_id, api_key_id, api_key_name, operation, model, prompt, size,
			output_format, image_url, b64_json, mime_type, source_image_url,
			node_id, root_node_id, parent_node_id, status, error_message, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id, created_at
	`, item.UserID, item.APIKeyID, item.APIKeyName, item.Operation, item.Model, item.Prompt, item.Size,
		item.OutputFormat, item.ImageURL, item.B64JSON, item.MimeType, item.SourceImageURL,
		item.NodeID, item.RootNodeID, item.ParentNodeID, item.Status, item.ErrorMessage, item.CreatedAt,
	).Scan(&item.ID, &item.CreatedAt)
}

func (r *imageCanvasHistoryRepository) DeleteByID(ctx context.Context, userID, id int64) error {
	_, err := r.db.ExecContext(ctx, `
		WITH RECURSIVE target AS (
			SELECT node_id FROM image_canvas_histories WHERE id = $1 AND user_id = $2
		), descendants AS (
			SELECT node_id FROM target WHERE node_id <> ''
			UNION ALL
			SELECT h.node_id
			FROM image_canvas_histories h
			JOIN descendants d ON h.parent_node_id = d.node_id
			WHERE h.user_id = $2 AND h.deleted_at IS NULL AND h.node_id <> ''
		)
		UPDATE image_canvas_histories
		SET deleted_at = NOW(), image_url = '', b64_json = '', source_image_url = ''
		WHERE user_id = $2 AND deleted_at IS NULL AND (
			id = $1 OR node_id IN (SELECT node_id FROM descendants)
		)
	`, id, userID)
	return err
}

func (r *imageCanvasHistoryRepository) CleanupExpiredImages(ctx context.Context, before time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE image_canvas_histories
		SET image_url = '', b64_json = '', source_image_url = ''
		WHERE created_at < $1 AND (image_url <> '' OR b64_json <> '' OR source_image_url <> '')
	`, before)
	return err
}

func (r *imageCanvasHistoryRepository) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]service.ImageCanvasHistory, *pagination.PaginationResult, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM image_canvas_histories WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, api_key_id, api_key_name, operation, model, prompt, size,
		       output_format, image_url, b64_json, mime_type, source_image_url,
		       node_id, root_node_id, parent_node_id, status, error_message, created_at
		FROM image_canvas_histories
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]service.ImageCanvasHistory, 0)
	for rows.Next() {
		var item service.ImageCanvasHistory
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.APIKeyID, &item.APIKeyName, &item.Operation, &item.Model, &item.Prompt, &item.Size,
			&item.OutputFormat, &item.ImageURL, &item.B64JSON, &item.MimeType, &item.SourceImageURL,
			&item.NodeID, &item.RootNodeID, &item.ParentNodeID, &item.Status, &item.ErrorMessage, &item.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	pages := int(math.Ceil(float64(total) / float64(params.PageSize)))
	if pages < 1 {
		pages = 1
	}
	return items, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.PageSize, Pages: pages}, nil
}
