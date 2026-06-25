package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	SettingKeyRequestResponseCaptureEnabled      = "request_response_capture_enabled"
	SettingKeyRequestResponseCaptureMaxBodyBytes = "request_response_capture_max_body_bytes"
)

const DefaultRequestResponseCaptureMaxBodyBytes = 64 * 1024

type RequestResponseCaptureSettings struct {
	Enabled      bool `json:"enabled"`
	MaxBodyBytes int  `json:"max_body_bytes"`
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
	RequestBodyBytes  int       `json:"request_body_bytes"`
	ResponseBodyBytes int       `json:"response_body_bytes"`
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
		return RequestResponseCaptureSettings{Enabled: false, MaxBodyBytes: DefaultRequestResponseCaptureMaxBodyBytes}
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
	maxBodyBytes := DefaultRequestResponseCaptureMaxBodyBytes
	enabled := false
	if cfg != nil {
		enabled = cfg.Gateway.RequestResponseCapture.Enabled
		if cfg.Gateway.RequestResponseCapture.MaxBodyBytes > 0 {
			maxBodyBytes = cfg.Gateway.RequestResponseCapture.MaxBodyBytes
		}
	}
	return RequestResponseCaptureSettings{Enabled: enabled, MaxBodyBytes: maxBodyBytes}
}

func normalizeRequestResponseCaptureSettings(in RequestResponseCaptureSettings) RequestResponseCaptureSettings {
	if in.MaxBodyBytes <= 0 {
		in.MaxBodyBytes = DefaultRequestResponseCaptureMaxBodyBytes
	}
	if in.MaxBodyBytes > 1024*1024 {
		in.MaxBodyBytes = 1024 * 1024
	}
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
