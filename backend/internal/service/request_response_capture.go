package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	SettingKeyRequestResponseCaptureEnabled      = "request_response_capture_enabled"
	SettingKeyRequestResponseCaptureGroupID      = "request_response_capture_group_id"
	SettingKeyRequestResponseCaptureMaxBodyBytes = "request_response_capture_max_body_bytes"
)

const DefaultRequestResponseCaptureMaxBodyBytes = 0

type RequestResponseCaptureSettings struct {
	Enabled bool  `json:"enabled"`
	GroupID int64 `json:"group_id"`
	// MaxBodyBytes is retained for API compatibility. Capture is always unlimited and returns 0.
	MaxBodyBytes int `json:"max_body_bytes"`
}

func (s RequestResponseCaptureSettings) CapturesGroup(groupID *int64) bool {
	return s.GroupID == 0 || (groupID != nil && *groupID == s.GroupID)
}

type RequestResponseCaptureSettingsReader interface {
	GetRequestResponseCaptureSettings(ctx context.Context) RequestResponseCaptureSettings
}

type RequestResponseLog struct {
	ID                int64     `json:"id"`
	RequestID         string    `json:"request_id"`
	UserID            int64     `json:"user_id"`
	APIKeyID          int64     `json:"api_key_id"`
	GroupID           *int64    `json:"group_id,omitempty"`
	Method            string    `json:"method"`
	Path              string    `json:"path"`
	Endpoint          string    `json:"endpoint"`
	Model             string    `json:"model"`
	Stream            bool      `json:"stream"`
	StatusCode        int       `json:"status_code"`
	RequestBody       string    `json:"request_body"`
	ResponseBody      string    `json:"response_body"`
	RequestTruncated  bool      `json:"request_truncated"`
	ResponseTruncated bool      `json:"response_truncated"`
	RequestBodyBytes  int64     `json:"request_body_bytes"`
	ResponseBodyBytes int64     `json:"response_body_bytes"`
	DurationMs        int64     `json:"duration_ms"`
	UserAgent         string    `json:"user_agent"`
	IPAddress         string    `json:"ip_address"`
	CreatedAt         time.Time `json:"created_at"`
}

type RequestResponseLogFilters struct {
	UserID    int64
	APIKeyID  int64
	GroupID   int64
	Endpoint  string
	Model     string
	Path      string
	Search    string
	StartTime *time.Time
	EndTime   *time.Time
}

type RequestResponseLogRepository interface {
	Create(ctx context.Context, log *RequestResponseLog) error
	List(ctx context.Context, page, pageSize int, filters RequestResponseLogFilters) ([]RequestResponseLog, int64, error)
	ListForExport(ctx context.Context, filters RequestResponseLogFilters, limit int) ([]RequestResponseLog, error)
	GetByID(ctx context.Context, id int64) (*RequestResponseLog, error)
}

type RequestResponseCaptureService struct {
	repo           RequestResponseLogRepository
	cfg            *config.Config
	settingsReader RequestResponseCaptureSettingsReader
}

func NewRequestResponseCaptureService(repo RequestResponseLogRepository, cfg *config.Config, settingsReader RequestResponseCaptureSettingsReader) *RequestResponseCaptureService {
	return &RequestResponseCaptureService{repo: repo, cfg: cfg, settingsReader: settingsReader}
}

func (s *RequestResponseCaptureService) IsEnabled(ctx context.Context) bool {
	return s.Settings(ctx).Enabled
}

func (s *RequestResponseCaptureService) MaxBodyBytes(ctx context.Context) int {
	return s.Settings(ctx).MaxBodyBytes
}

func (s *RequestResponseCaptureService) Settings(ctx context.Context) RequestResponseCaptureSettings {
	if s == nil {
		return RequestResponseCaptureSettings{Enabled: false}
	}
	if s.settingsReader != nil {
		return s.settingsReader.GetRequestResponseCaptureSettings(ctx)
	}
	return requestResponseCaptureSettingsFromConfig(s.cfg)
}

func (s *RequestResponseCaptureService) Create(ctx context.Context, log *RequestResponseLog) error {
	if s == nil || s.repo == nil || log == nil {
		return nil
	}
	return s.repo.Create(ctx, log)
}

func (s *RequestResponseCaptureService) List(ctx context.Context, page, pageSize int, filters RequestResponseLogFilters) ([]RequestResponseLog, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, nil
	}
	return s.repo.List(ctx, page, pageSize, filters)
}

func (s *RequestResponseCaptureService) GetByID(ctx context.Context, id int64) (*RequestResponseLog, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSettingNotFound
	}
	return s.repo.GetByID(ctx, id)
}

func (s *RequestResponseCaptureService) ListForExport(ctx context.Context, filters RequestResponseLogFilters, limit int) ([]RequestResponseLog, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.ListForExport(ctx, filters, limit)
}

func requestResponseCaptureSettingsFromConfig(cfg *config.Config) RequestResponseCaptureSettings {
	enabled := true
	var groupID int64
	if cfg != nil {
		enabled = cfg.Gateway.RequestResponseCapture.Enabled
		groupID = cfg.Gateway.RequestResponseCapture.GroupID
	}
	return normalizeRequestResponseCaptureSettings(RequestResponseCaptureSettings{Enabled: enabled, GroupID: groupID})
}

func normalizeRequestResponseCaptureSettings(in RequestResponseCaptureSettings) RequestResponseCaptureSettings {
	if in.GroupID < 0 {
		in.GroupID = 0
	}
	in.MaxBodyBytes = DefaultRequestResponseCaptureMaxBodyBytes
	return in
}

func parseBoolSetting(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}
