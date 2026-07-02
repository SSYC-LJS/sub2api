package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	ImageCanvasOperationGenerate = "generate"
	ImageCanvasOperationEdit     = "edit"
)

type ImageCanvasHistory struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	APIKeyID       int64     `json:"api_key_id"`
	APIKeyName     string    `json:"api_key_name"`
	Operation      string    `json:"operation"`
	Model          string    `json:"model"`
	Prompt         string    `json:"prompt"`
	Size           string    `json:"size"`
	OutputFormat   string    `json:"output_format"`
	ImageURL       string    `json:"image_url,omitempty"`
	B64JSON        string    `json:"b64_json,omitempty"`
	MimeType       string    `json:"mime_type"`
	SourceImageURL string    `json:"source_image_url,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateImageCanvasHistoryRequest struct {
	APIKeyID       int64  `json:"api_key_id"`
	Operation      string `json:"operation"`
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size"`
	OutputFormat   string `json:"output_format"`
	ImageURL       string `json:"image_url"`
	B64JSON        string `json:"b64_json"`
	MimeType       string `json:"mime_type"`
	SourceImageURL string `json:"source_image_url"`
}

type ImageCanvasHistoryRepository interface {
	Create(ctx context.Context, item *ImageCanvasHistory) error
	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageCanvasHistory, *pagination.PaginationResult, error)
}

type ImageCanvasService struct {
	repo    ImageCanvasHistoryRepository
	apiKeys *APIKeyService
}

func NewImageCanvasService(repo ImageCanvasHistoryRepository, apiKeys *APIKeyService) *ImageCanvasService {
	return &ImageCanvasService{repo: repo, apiKeys: apiKeys}
}

func (s *ImageCanvasService) CreateHistory(ctx context.Context, userID int64, req CreateImageCanvasHistoryRequest) (*ImageCanvasHistory, error) {
	if req.APIKeyID <= 0 {
		return nil, infraerrors.BadRequest("IMAGE_CANVAS_INVALID_API_KEY", "请选择要使用的 API Key")
	}
	key, err := s.apiKeys.GetByID(ctx, req.APIKeyID)
	if err != nil || key == nil || key.UserID != userID {
		return nil, infraerrors.NotFound("IMAGE_CANVAS_API_KEY_NOT_FOUND", "API Key 不存在或无权使用")
	}
	operation := strings.TrimSpace(req.Operation)
	if operation == "" {
		operation = ImageCanvasOperationGenerate
	}
	if operation != ImageCanvasOperationGenerate && operation != ImageCanvasOperationEdit {
		return nil, infraerrors.BadRequest("IMAGE_CANVAS_INVALID_OPERATION", "不支持的图片操作类型")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, infraerrors.BadRequest("IMAGE_CANVAS_MODEL_REQUIRED", "请选择生图模型")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, infraerrors.BadRequest("IMAGE_CANVAS_PROMPT_REQUIRED", "请输入提示词")
	}
	imageURL := strings.TrimSpace(req.ImageURL)
	b64 := strings.TrimSpace(req.B64JSON)
	if imageURL == "" && b64 == "" {
		return nil, infraerrors.BadRequest("IMAGE_CANVAS_IMAGE_REQUIRED", "缺少生成后的图片内容")
	}
	format := strings.ToLower(strings.TrimSpace(req.OutputFormat))
	if format == "" {
		format = "png"
	}
	mime := strings.TrimSpace(req.MimeType)
	if mime == "" {
		mime = "image/" + format
	}
	item := &ImageCanvasHistory{
		UserID:         userID,
		APIKeyID:       req.APIKeyID,
		APIKeyName:     key.Name,
		Operation:      operation,
		Model:          model,
		Prompt:         prompt,
		Size:           strings.TrimSpace(req.Size),
		OutputFormat:   format,
		ImageURL:       imageURL,
		B64JSON:        b64,
		MimeType:       mime,
		SourceImageURL: strings.TrimSpace(req.SourceImageURL),
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *ImageCanvasService) ListHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]ImageCanvasHistory, *pagination.PaginationResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 200 {
		params.PageSize = 100
	}
	params.SortBy = "created_at"
	params.SortOrder = pagination.SortOrderDesc
	return s.repo.ListByUser(ctx, userID, params)
}
